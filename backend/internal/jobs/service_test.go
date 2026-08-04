package jobs

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/database/tursohttp"
	"coreloop/backend/internal/qstash"
	"coreloop/backend/internal/store"
)

type jobRoundTripFunc func(*http.Request) (*http.Response, error)

func (function jobRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestRunAcknowledgesAJobThatIsNotCurrentlyLeasable(t *testing.T) {
	for _, state := range []string{"queued", "leased", "completed", "blocked_quota", "failed"} {
		t.Run(state, func(t *testing.T) {
			requestCount := 0
			client := &http.Client{Transport: jobRoundTripFunc(func(*http.Request) (*http.Response, error) {
				requestCount++
				body := `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
				if requestCount == 2 {
					body = `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"state"}],"rows":[[{"type":"text","value":"` + state + `"}]],"affected_row_count":0,"last_insert_rowid":null}}}]}`
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(body)),
					Header:     make(http.Header),
				}, nil
			})}
			database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()

			service := New(store.New(database), nil, nil, nil, nil, "https://coreloop.example")
			err = service.Run(context.Background(), "job_retry", "qstash:retry")
			if err != nil {
				t.Fatalf("job in state %q must be acknowledged: %v", state, err)
			}
			if requestCount != 2 {
				t.Fatalf("request count = %d, want 2", requestCount)
			}
		})
	}
}

func TestRunReturnsAContextualErrorForAMissingJob(t *testing.T) {
	requestCount := 0
	client := &http.Client{Transport: jobRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requestCount++
		body := `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
		if requestCount == 2 {
			body = `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"state"}],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	service := New(store.New(database), nil, nil, nil, nil, "https://coreloop.example")
	err = service.Run(context.Background(), "job_missing", "qstash:retry")
	if !errors.Is(err, sql.ErrNoRows) || !strings.Contains(err.Error(), "lease job") {
		t.Fatalf("missing job error = %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
}

func TestLeaseJobDoesNotRequireATransactionBaton(t *testing.T) {
	requestCount := 0
	var received []byte
	client := &http.Client{Transport: jobRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount++
		received, _ = io.ReadAll(request.Body)
		body := `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[` +
			`{"name":"id"},{"name":"sequence"},{"name":"user_id"},{"name":"assignment_id"},` +
			`{"name":"job_type"},{"name":"state"},{"name":"due_at"},{"name":"attempt_count"},` +
			`{"name":"max_attempts"},{"name":"idempotency_key"},{"name":"payload_json"}` +
			`],"rows":[[` +
			`{"type":"text","value":"job_ready"},` +
			`{"type":"integer","value":"7"},` +
			`{"type":"text","value":""},` +
			`{"type":"text","value":""},` +
			`{"type":"text","value":"recover"},` +
			`{"type":"text","value":"leased"},` +
			`{"type":"text","value":"2026-08-04T12:45:00Z"},` +
			`{"type":"integer","value":"1"},` +
			`{"type":"integer","value":"5"},` +
			`{"type":"text","value":"recover:test"},` +
			`{"type":"text","value":"{}"}` +
			`]],"affected_row_count":1,"last_insert_rowid":null}}},` +
			`{"type":"ok","response":{"type":"close"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	job, err := store.New(database).LeaseJob(context.Background(), "job_ready", "qstash:test", time.Now())
	if err != nil {
		t.Fatalf("lease job: %v", err)
	}
	if job.ID != "job_ready" || job.State != "leased" {
		t.Fatalf("leased job = %#v", job)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	if !bytes.Contains(received, []byte("UPDATE job_queue")) || !bytes.Contains(received, []byte("RETURNING id")) {
		t.Fatalf("lease was not sent as one atomic UPDATE RETURNING statement: %s", received)
	}
	if bytes.Contains(received, []byte("BEGIN")) {
		t.Fatalf("lease unexpectedly opened a transaction: %s", received)
	}
}

func TestSchedulerPublishesOneChronologicalJobPerTick(t *testing.T) {
	if publishableJobsPerTick != 1 {
		t.Fatalf("publishable jobs per tick = %d, want 1", publishableJobsPerTick)
	}
}

func TestCompletedJobDispatchesTheNextDueJob(t *testing.T) {
	databaseRequestCount := 0
	databaseClient := &http.Client{Transport: jobRoundTripFunc(func(*http.Request) (*http.Response, error) {
		databaseRequestCount++
		body := emptyTursoResult()
		switch databaseRequestCount {
		case 1:
			body = tursoJobResult("job_current", "recover", "leased")
		case 7:
			body = tursoJobResult("job_next", "generate_lesson", "queued")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", databaseClient)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var publishedBody string
	publisherClient := &http.Client{Transport: jobRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		publishedBody = string(body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}
	service := New(store.New(database), nil, nil, qstash.NewPublisher("secret", publisherClient), nil, "https://coreloop.example")
	if err := service.Run(context.Background(), "job_current", "qstash:test"); err != nil {
		t.Fatal(err)
	}
	if databaseRequestCount != 7 {
		t.Fatalf("database request count = %d, want 7", databaseRequestCount)
	}
	if publishedBody != `{"job_id":"job_next"}` {
		t.Fatalf("published body = %q, want next job", publishedBody)
	}
}

func emptyTursoResult() string {
	return `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":1,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
}

func tursoJobResult(jobID, jobType, state string) string {
	return `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[` +
		`{"name":"id"},{"name":"sequence"},{"name":"user_id"},{"name":"assignment_id"},` +
		`{"name":"job_type"},{"name":"state"},{"name":"due_at"},{"name":"attempt_count"},` +
		`{"name":"max_attempts"},{"name":"idempotency_key"},{"name":"payload_json"}` +
		`],"rows":[[` +
		`{"type":"text","value":"` + jobID + `"},` +
		`{"type":"integer","value":"7"},` +
		`{"type":"text","value":""},` +
		`{"type":"text","value":""},` +
		`{"type":"text","value":"` + jobType + `"},` +
		`{"type":"text","value":"` + state + `"},` +
		`{"type":"text","value":"2026-08-04T13:00:00Z"},` +
		`{"type":"integer","value":"1"},` +
		`{"type":"integer","value":"5"},` +
		`{"type":"text","value":"test-key"},` +
		`{"type":"text","value":"{}"}` +
		`]],"affected_row_count":1,"last_insert_rowid":null}}},` +
		`{"type":"ok","response":{"type":"close"}}]}`
}
