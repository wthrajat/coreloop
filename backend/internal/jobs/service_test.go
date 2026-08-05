package jobs

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/database/tursohttp"
	"coreloop/backend/internal/qstash"
	"coreloop/backend/internal/store"
)

type jobRoundTripFunc func(*http.Request) (*http.Response, error)

func (function jobRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Body == nil {
		return function(request)
	}
	requestBody, _ := io.ReadAll(request.Body)
	request.Body = io.NopCloser(bytes.NewReader(requestBody))
	response, err := function(request)
	if err != nil || response == nil || !startsFreshTursoStream(requestBody) {
		return response, err
	}
	return prependForeignKeyResult(response)
}

func startsFreshTursoStream(body []byte) bool {
	var payload struct {
		Baton    string `json:"baton"`
		Requests []struct {
			Statement *struct {
				SQL string `json:"sql"`
			} `json:"stmt"`
		} `json:"requests"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.Baton != "" ||
		len(payload.Requests) == 0 || payload.Requests[0].Statement == nil {
		return false
	}
	return payload.Requests[0].Statement.SQL == "PRAGMA foreign_keys=ON"
}

func prependForeignKeyResult(response *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	response.Body.Close()
	var payload struct {
		Results []json.RawMessage `json:"results"`
		Baton   string            `json:"baton,omitempty"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		response.Body = io.NopCloser(bytes.NewReader(body))
		return response, nil
	}
	foreignKeys := json.RawMessage(`{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}`)
	payload.Results = append([]json.RawMessage{foreignKeys}, payload.Results...)
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	response.Body = io.NopCloser(bytes.NewReader(encoded))
	return response, nil
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
			wantRequests := 2
			if state == "queued" {
				wantRequests = 3
			}
			if state == "completed" || state == "blocked_quota" || state == "failed" {
				wantRequests = 3
			}
			if requestCount != wantRequests {
				t.Fatalf("request count = %d, want %d", requestCount, wantRequests)
			}
		})
	}
}

func TestCompletedDuplicateWakeResumesTheQueue(t *testing.T) {
	databaseRequestCount := 0
	databaseClient := &http.Client{Transport: jobRoundTripFunc(func(*http.Request) (*http.Response, error) {
		databaseRequestCount++
		body := emptyTursoResult()
		switch databaseRequestCount {
		case 1:
			body = `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
		case 2:
			body = `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"state"}],"rows":[[{"type":"text","value":"completed"}]],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
		case 3:
			body = tursoJobResult("job_next", "deliver_lesson", "queued")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", databaseClient)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var publishedBody string
	var publishedDeduplicationID string
	publisherClient := &http.Client{Transport: jobRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		publishedBody = string(body)
		publishedDeduplicationID = request.Header.Get("Upstash-Deduplication-Id")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"messageId":"msg_next","deduplicated":false}`)),
			Header:     make(http.Header),
		}, nil
	})}
	service := New(store.New(database), nil, nil, qstash.NewPublisher("secret", publisherClient), nil, "https://coreloop.example")
	service.now = func() time.Time {
		return time.Date(2026, time.August, 5, 12, 4, 0, 0, time.UTC)
	}
	if err := service.Run(context.Background(), "job_completed", "qstash:duplicate"); err != nil {
		t.Fatal(err)
	}
	if databaseRequestCount != 3 {
		t.Fatalf("database request count = %d, want 3", databaseRequestCount)
	}
	if publishedBody != `{"job_id":"job_next"}` {
		t.Fatalf("published body = %q", publishedBody)
	}
	if publishedDeduplicationID != "dispatch-job_next-1-20260805T1200" {
		t.Fatalf("published deduplication id = %q", publishedDeduplicationID)
	}
}

func TestExhaustedQueuedWakeIsFinalizedAndResumesTheQueue(t *testing.T) {
	databaseRequestCount := 0
	var finalizationRequest []byte
	databaseClient := &http.Client{Transport: jobRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		databaseRequestCount++
		body := emptyTursoResult()
		switch databaseRequestCount {
		case 1:
			body = `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
		case 2:
			body = `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"state"}],"rows":[[{"type":"text","value":"queued"}]],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
		case 3:
			finalizationRequest, _ = io.ReadAll(request.Body)
		case 4:
			body = tursoJobResult("job_next", "generate_lesson", "queued")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
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
			Body:       io.NopCloser(strings.NewReader(`{"messageId":"msg_next","deduplicated":false}`)),
			Header:     make(http.Header),
		}, nil
	})}
	service := New(store.New(database), nil, nil, qstash.NewPublisher("secret", publisherClient), nil, "https://coreloop.example")
	if err := service.Run(context.Background(), "job_exhausted", "qstash:stale"); err != nil {
		t.Fatal(err)
	}
	if databaseRequestCount != 4 {
		t.Fatalf("database request count = %d, want 4", databaseRequestCount)
	}
	finalizationSQL := firstTursoSQL(t, finalizationRequest)
	if !strings.Contains(finalizationSQL, "state='failed'") || !strings.Contains(finalizationSQL, "attempt_count>=max_attempts") {
		t.Fatalf("exhausted job was not terminalized safely: %s", finalizationSQL)
	}
	if publishedBody != `{"job_id":"job_next"}` {
		t.Fatalf("published body = %q", publishedBody)
	}
}

func TestQueueContinuationKeepsAPublishFailureRetryable(t *testing.T) {
	databaseClient := &http.Client{Transport: jobRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(tursoJobResult("job_next", "deliver_lesson", "queued"))),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", databaseClient)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	publisherClient := &http.Client{Transport: jobRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "503 Service Unavailable",
			Body:       io.NopCloser(strings.NewReader(`{"error":"temporarily unavailable"}`)),
			Header:     make(http.Header),
		}, nil
	})}
	service := New(store.New(database), nil, nil, qstash.NewPublisher("secret", publisherClient), nil, "https://coreloop.example")
	err = service.continueQueue(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dispatch next chronological job") || !strings.Contains(err.Error(), "503") {
		t.Fatalf("continuation error = %v", err)
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

func TestDispatchDeduplicationIsScopedToTheDurableAttempt(t *testing.T) {
	first := dispatchDeduplicationID("job_retry-safe", 0)
	retry := dispatchDeduplicationID("job_retry-safe", 1)
	if first != "dispatch-job_retry-safe-0" {
		t.Fatalf("first deduplication id = %q", first)
	}
	if retry != "dispatch-job_retry-safe-1" || retry == first {
		t.Fatalf("retry deduplication id = %q, first = %q", retry, first)
	}
	if strings.Contains(first, ":") || strings.Contains(retry, ":") {
		t.Fatalf("QStash deduplication ids must not contain colons: %q, %q", first, retry)
	}
}

func TestScheduledDispatchDeduplicationRetriesAQueuedAttemptOnTheNextTick(t *testing.T) {
	firstTick := time.Date(2026, time.August, 5, 12, 4, 0, 0, time.UTC)
	sameWindow := firstTick.Add(5 * time.Minute)
	nextWindow := firstTick.Add(10 * time.Minute)

	first := scheduledDispatchDeduplicationID("job_radar", 0, firstTick)
	same := scheduledDispatchDeduplicationID("job_radar", 0, sameWindow)
	next := scheduledDispatchDeduplicationID("job_radar", 0, nextWindow)
	if first != "dispatch-job_radar-0-20260805T1200" || same != first {
		t.Fatalf("same-window IDs = %q and %q", first, same)
	}
	if next != "dispatch-job_radar-0-20260805T1210" || next == first {
		t.Fatalf("next-window ID = %q, first = %q", next, first)
	}
	if strings.Contains(first, ":") || strings.Contains(next, ":") {
		t.Fatalf("QStash deduplication IDs must not contain colons: %q, %q", first, next)
	}
}

func TestSchedulerRepublishesTheSameQueuedRadarJobInTheNextWindow(t *testing.T) {
	databaseClient := &http.Client{Transport: jobRoundTripFunc(func(
		*http.Request,
	) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				tursoJobResult("job_radar", "deliver_radar", "queued"),
			)),
			Header: make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", databaseClient)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var publishedIDs []string
	publisherClient := &http.Client{Transport: jobRoundTripFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		publishedIDs = append(
			publishedIDs,
			request.Header.Get("Upstash-Deduplication-Id"),
		)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"messageId":"msg_radar","deduplicated":false}`,
			)),
			Header: make(http.Header),
		}, nil
	})}
	service := New(
		store.New(database),
		nil,
		nil,
		qstash.NewPublisher("secret", publisherClient),
		nil,
		"https://coreloop.example",
	)
	now := time.Date(2026, time.August, 5, 12, 4, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if err := service.dispatchNextDueJob(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(10 * time.Minute)
	if err := service.dispatchNextDueJob(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"dispatch-job_radar-1-20260805T1200",
		"dispatch-job_radar-1-20260805T1210",
	}
	if !reflect.DeepEqual(publishedIDs, want) {
		t.Fatalf("published IDs = %#v, want %#v", publishedIDs, want)
	}
}

func TestJobQueueSummaryReportsTheStageAndOldestDueTime(t *testing.T) {
	client := &http.Client{Transport: jobRoundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[` +
			`{"name":"job_type"},{"name":"state"},{"name":"count(*)"},{"name":"min(due_at)"}` +
			`],"rows":[[` +
			`{"type":"text","value":"generate_lesson"},{"type":"text","value":"queued"},` +
			`{"type":"integer","value":"2"},{"type":"text","value":"2026-08-04T13:00:00Z"}` +
			`],[` +
			`{"type":"text","value":"deliver_lesson"},{"type":"text","value":"blocked_quota"},` +
			`{"type":"integer","value":"1"},{"type":"text","value":"2026-08-04T14:00:00Z"}` +
			`]],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	summary, err := store.New(database).JobQueueSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(summary) != 2 {
		t.Fatalf("summary = %#v", summary)
	}
	if summary[0].Type != "generate_lesson" || summary[0].State != "queued" || summary[0].Count != 2 {
		t.Fatalf("generation summary = %#v", summary[0])
	}
	if got := summary[1].OldestDueAt.UTC().Format(time.RFC3339); got != "2026-08-04T14:00:00Z" {
		t.Fatalf("oldest delivery due time = %q", got)
	}
}

func TestRecoveryTerminalizesExhaustedJobsAndPreservesQuotaRetries(t *testing.T) {
	var requests [][]byte
	client := &http.Client{Transport: jobRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		requests = append(requests, body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(emptyTursoResult())),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := store.New(database).RecoverJobs(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 6 {
		t.Fatalf("recovery request count = %d, want 6", len(requests))
	}
	exhaustedSQL := firstTursoSQL(t, requests[0])
	if !strings.Contains(exhaustedSQL, "state='failed'") || !strings.Contains(exhaustedSQL, "attempt_count>=max_attempts") {
		t.Fatalf("exhausted recovery query = %s", exhaustedSQL)
	}
	leaseRecoverySQL := firstTursoSQL(t, requests[1])
	if !strings.Contains(leaseRecoverySQL, "attempt_count<max_attempts") {
		t.Fatalf("lease recovery query = %s", leaseRecoverySQL)
	}
	quotaRecoverySQL := firstTursoSQL(t, requests[2])
	if !strings.Contains(quotaRecoverySQL, "MIN(attempt_count,max_attempts-1)") {
		t.Fatalf("quota recovery query = %s", quotaRecoverySQL)
	}
}

func TestPublishableJobsExcludeExhaustedQueuedRows(t *testing.T) {
	var requestBody []byte
	client := &http.Client{Transport: jobRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestBody, _ = io.ReadAll(request.Body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(emptyTursoResult())),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	jobs, err := store.New(database).PublishableJobs(context.Background(), time.Now(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("publishable jobs = %#v", jobs)
	}
	publishableSQL := firstTursoSQL(t, requestBody)
	if !strings.Contains(publishableSQL, "attempt_count<max_attempts") {
		t.Fatalf("publishable query can select exhausted rows: %s", publishableSQL)
	}
}

func TestQuotaFailureDoesNotConsumeTheOrdinaryAttemptBudget(t *testing.T) {
	var requestBody []byte
	client := &http.Client{Transport: jobRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestBody, _ = io.ReadAll(request.Body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(emptyTursoResult())),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	job := store.Job{ID: "job_quota", AttemptCount: 5, MaxAttempts: 5}
	if err := store.New(database).FailJob(context.Background(), job, "worker", "ai_quota_exhausted", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	quotaFailureSQL := firstTursoSQL(t, requestBody)
	if !strings.Contains(quotaFailureSQL, "state='blocked_quota'") || !strings.Contains(quotaFailureSQL, "MAX(attempt_count-1,0)") {
		t.Fatalf("quota failure consumed the ordinary attempt budget: %s", quotaFailureSQL)
	}
}

func TestFailedJobStateMatchesDurableRetryRules(t *testing.T) {
	job := store.Job{AttemptCount: 2, MaxAttempts: 5}
	if state := failedJobState(job, false); state != "queued" {
		t.Fatalf("retryable state = %q", state)
	}
	job.AttemptCount = job.MaxAttempts
	if state := failedJobState(job, false); state != "failed" {
		t.Fatalf("exhausted state = %q", state)
	}
	if state := failedJobState(job, true); state != "blocked_quota" {
		t.Fatalf("quota state = %q", state)
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
		case 9:
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
	if databaseRequestCount != 9 {
		t.Fatalf("database request count = %d, want 9", databaseRequestCount)
	}
	if publishedBody != `{"job_id":"job_next"}` {
		t.Fatalf("published body = %q, want next job", publishedBody)
	}
}

func TestJobExecutionDeadlinePreservesFinalizationTime(t *testing.T) {
	parentDeadline := time.Now().Add(30 * time.Second)
	parent, cancelParent := context.WithDeadline(context.Background(), parentDeadline)
	defer cancelParent()

	execution, cancelExecution := jobExecutionContext(parent)
	defer cancelExecution()
	executionDeadline, ok := execution.Deadline()
	if !ok {
		t.Fatal("execution context has no deadline")
	}
	reserve := parentDeadline.Sub(executionDeadline)
	if reserve < 7*time.Second || reserve > 9*time.Second {
		t.Fatalf("finalization reserve = %s, want approximately 8s", reserve)
	}
}

func TestLessonDeliveryTextAddsOneEscapedRecallPrompt(t *testing.T) {
	message, err := lessonDeliveryText("<b>Part 1/2</b>\nLesson", "What does A < B mean?", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "Quick recall") || !strings.Contains(message, "A &lt; B") {
		t.Fatalf("recall prompt was not safely added: %s", message)
	}
	second, err := lessonDeliveryText("<b>Part 2/2</b>\nLesson", "Question", false)
	if err != nil || strings.Contains(second, "Quick recall") {
		t.Fatalf("recall prompt leaked into a later part: %q, %v", second, err)
	}
}

func emptyTursoResult() string {
	return `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":1,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
}

func firstTursoSQL(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Requests []struct {
			Statement *struct {
				SQL string `json:"sql"`
			} `json:"stmt"`
		} `json:"requests"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode Turso request: %v", err)
	}
	if len(payload.Requests) == 0 || payload.Requests[0].Statement == nil {
		t.Fatalf("Turso request has no SQL statement: %s", body)
	}
	index := 0
	if payload.Requests[0].Statement.SQL == "PRAGMA foreign_keys=ON" {
		index = 1
	}
	if len(payload.Requests) <= index || payload.Requests[index].Statement == nil {
		t.Fatalf("Turso request has no application SQL statement: %s", body)
	}
	return payload.Requests[index].Statement.SQL
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
