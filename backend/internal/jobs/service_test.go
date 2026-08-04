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

	"coreloop/backend/internal/database/tursohttp"
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
				body := `{"baton":"test-transaction","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
				if requestCount == 3 {
					body = `{"baton":"test-transaction","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"state"}],"rows":[[{"type":"text","value":"` + state + `"}]],"affected_row_count":0,"last_insert_rowid":null}}}]}`
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
			if requestCount != 4 {
				t.Fatalf("request count = %d, want 4", requestCount)
			}
		})
	}
}

func TestRunReturnsAContextualErrorForAMissingJob(t *testing.T) {
	requestCount := 0
	client := &http.Client{Transport: jobRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requestCount++
		body := `{"baton":"test-transaction","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
		if requestCount == 3 {
			body = `{"baton":"test-transaction","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"state"}],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
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
}
