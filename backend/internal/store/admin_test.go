package store

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"coreloop/backend/internal/database/tursohttp"
)

func TestOperationsLoadsFailedJobsAndAttemptHistoryWithoutNPlusOne(t *testing.T) {
	requestCount := 0
	var requestBodies [][]byte
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		requestCount++
		body, _ := io.ReadAll(request.Body)
		requestBodies = append(requestBodies, body)
		return tursoTestResponse(operationsTestResult(requestCount)), nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	operations, err := New(database).Operations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if requestCount != 5 {
		t.Fatalf("requests = %d, want five set-based queries", requestCount)
	}
	if operations.Failed != 1 || len(operations.FailedJobs) != 1 {
		t.Fatalf("unexpected operations summary: %#v", operations)
	}
	failed := operations.FailedJobs[0]
	if failed.ID != "job_failed" || failed.Type != "generate_lesson" ||
		failed.LastErrorCode != "ai_invalid_output" || len(failed.Failures) != 2 {
		t.Fatalf("unexpected failed job: %#v", failed)
	}
	if failed.Failures[0].AttemptCount != 5 ||
		failed.Failures[0].NextState != "failed" ||
		failed.Failures[1].AttemptCount != 4 ||
		failed.Failures[1].NextState != "queued" {
		t.Fatalf("attempt history is not newest first: %#v", failed.Failures)
	}
	if len(operations.SourceHealth) != 1 ||
		operations.SourceHealth[0].PollState != "failed" ||
		operations.SourceHealth[0].LastErrorCode != "source_http_error" {
		t.Fatalf("unexpected source health: %#v", operations.SourceHealth)
	}
	combined := string(bytes.Join(requestBodies, []byte("\n")))
	if !strings.Contains(combined, "FROM job_failure_events event") ||
		!strings.Contains(combined, "LIMIT 50") ||
		!strings.Contains(combined, `failure_rank\u003c=20`) {
		t.Fatalf("failed-job queries are missing their bounded event join: %s", combined)
	}
}

func operationsTestResult(requestNumber int) string {
	var columns, rows string
	switch requestNumber {
	case 1:
		columns = `[{"name":"queued"},{"name":"leased"},{"name":"failed"},{"name":"blocked_quota"},{"name":"users"},{"name":"sources"}]`
		rows = `[[{"type":"integer","value":"2"},{"type":"integer","value":"0"},{"type":"integer","value":"1"},{"type":"integer","value":"0"},{"type":"integer","value":"1"},{"type":"integer","value":"41"}]]`
	case 2:
		columns = `[{"name":"id"},{"name":"created_at"},{"name":"attempt_count"}]`
		rows = `[]`
	case 3:
		columns = `[{"name":"id"},{"name":"job_type"},{"name":"created_at"},{"name":"failed_at"},{"name":"attempt_count"},{"name":"max_attempts"},{"name":"last_error_code"},{"name":"last_error_summary"},{"name":"failure_count"}]`
		rows = `[[{"type":"text","value":"job_failed"},{"type":"text","value":"generate_lesson"},{"type":"text","value":"2026-08-05T09:00:00Z"},{"type":"text","value":"2026-08-05T10:00:00Z"},{"type":"integer","value":"5"},{"type":"integer","value":"5"},{"type":"text","value":"ai_invalid_output"},{"type":"text","value":"Gemini returned invalid lesson output (HTTP 400)."},{"type":"integer","value":"2"}]]`
	case 4:
		columns = `[{"name":"job_id"},{"name":"attempt_count"},{"name":"error_code"},{"name":"error_summary"},{"name":"next_state"},{"name":"occurred_at"}]`
		rows = `[[{"type":"text","value":"job_failed"},{"type":"integer","value":"5"},{"type":"text","value":"ai_invalid_output"},{"type":"text","value":"Gemini returned invalid lesson output (HTTP 400)."},{"type":"text","value":"failed"},{"type":"text","value":"2026-08-05T10:00:00Z"}],[{"type":"text","value":"job_failed"},{"type":"integer","value":"4"},{"type":"text","value":"execution_timeout"},{"type":"text","value":"The job exceeded its execution deadline."},{"type":"text","value":"queued"},{"type":"text","value":"2026-08-05T09:30:00Z"}]]`
	case 5:
		columns = `[{"name":"id"},{"name":"publisher"},{"name":"fetch_method"},{"name":"source_role"},{"name":"last_poll_state"},{"name":"consecutive_failures"},{"name":"last_polled_at"},{"name":"last_success_at"},{"name":"last_error_code"},{"name":"last_error_summary"},{"name":"last_error_at"},{"name":"last_item_count"},{"name":"recent_items"}]`
		rows = `[[{"type":"text","value":"source_hacker_news"},{"type":"text","value":"Hacker News"},{"type":"text","value":"api"},{"type":"text","value":"community_discovery"},{"type":"text","value":"failed"},{"type":"integer","value":"2"},{"type":"text","value":"2026-08-05T10:10:00Z"},{"type":"text","value":"2026-08-05T09:00:00Z"},{"type":"text","value":"source_http_error"},{"type":"text","value":"The source returned HTTP 429."},{"type":"text","value":"2026-08-05T10:10:00Z"},{"type":"integer","value":"0"},{"type":"integer","value":"12"}]]`
	default:
		columns = `[]`
		rows = `[]`
	}
	return `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":` +
		columns + `,"rows":` + rows +
		`,"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
}
