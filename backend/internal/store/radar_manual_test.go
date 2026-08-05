package store

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/database/tursohttp"
)

type radarManualRoundTripFunc func(*http.Request) (*http.Response, error)

func (function radarManualRoundTripFunc) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	return function(request)
}

func TestEnqueueManualRadarBatchUsesSavedTargetWithoutDailyUsage(t *testing.T) {
	requestCount := 0
	var requests []string
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		requestCount++
		body, _ := io.ReadAll(request.Body)
		requests = append(requests, string(body))

		responseBody := tursoManualBatchResult(requestCount)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	dataStore := New(database)
	jobs, err := dataStore.EnqueueManualRadarBatch(
		context.Background(),
		"usr_owner",
		"batch_digest",
		"manual-radar:usr_owner:batch_digest:",
		time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("enqueue manual Radar batch: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("jobs = %d, want saved target of 3", len(jobs))
	}
	for _, batchJob := range jobs {
		job := batchJob.Job
		if !strings.HasPrefix(job.ID, "job_") {
			t.Fatalf("job id = %q", job.ID)
		}
		if job.State != "queued" || job.Type != "deliver_radar" {
			t.Fatalf("unexpected job: %#v", job)
		}
	}
	if requestCount != 7 {
		t.Fatalf("request count = %d, want 7", requestCount)
	}

	combined := []byte(strings.Join(requests, "\n"))
	for _, required := range [][]byte{
		[]byte(`"sql":"BEGIN"`),
		[]byte(`json_extract(jq.payload_json,'$.manual_batch_id')`),
		[]byte(`SELECT lp.radar_items_per_day`),
		[]byte(`rc.relevance_score\u003e=?`),
		[]byte(`SET status='qualified',updated_at=?`),
		[]byte(`"value":"deliver_radar"`),
		[]byte(`"value":"rad_first"`),
		[]byte(`"value":"rad_second"`),
		[]byte(`"value":"rad_third"`),
		[]byte(`"sql":"COMMIT"`),
	} {
		if !bytes.Contains(combined, required) {
			t.Fatalf("missing request fragment %q in %s", required, combined)
		}
	}
	if count := bytes.Count(combined, []byte(`"value":"deliver_radar"`)); count != 3 {
		t.Fatalf("deliver_radar inserts = %d, want 3", count)
	}
	if bytes.Contains(combined, []byte("radar_daily_usage")) ||
		bytes.Contains(combined, []byte("released_at")) {
		t.Fatalf("manual send consumed normal Radar cadence: %s", combined)
	}
}

func TestManualRadarBatchJobsLoadsDeliveryStateInOneQuery(t *testing.T) {
	requestCount := 0
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		requestCount++
		body := `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[` +
			`{"name":"id"},{"name":"sequence"},{"name":"user_id"},{"name":"assignment_id"},` +
			`{"name":"job_type"},{"name":"state"},{"name":"due_at"},{"name":"attempt_count"},` +
			`{"name":"max_attempts"},{"name":"idempotency_key"},{"name":"payload_json"},` +
			`{"name":"candidate_state"},{"name":"delivery_state"}],"rows":[[` +
			`{"type":"text","value":"job_radar_1"},` +
			`{"type":"integer","value":"17"},` +
			`{"type":"text","value":"usr_owner"},` +
			`{"type":"text","value":""},` +
			`{"type":"text","value":"deliver_radar"},` +
			`{"type":"text","value":"completed"},` +
			`{"type":"text","value":"2026-08-05T12:00:00Z"},` +
			`{"type":"integer","value":"1"},` +
			`{"type":"integer","value":"5"},` +
			`{"type":"text","value":"manual-radar:usr_owner:batch_digest:1"},` +
			`{"type":"text","value":"{\"candidate_id\":\"rad_first\",\"manual_batch_id\":\"batch_digest\",\"profile_target\":3,\"requested_count\":3}"},` +
			`{"type":"text","value":"delivered"},` +
			`{"type":"text","value":"delivered"}` +
			`]],"affected_row_count":0,"last_insert_rowid":null}}},` +
			`{"type":"ok","response":{"type":"close"}}]}`
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

	jobs, err := New(database).ManualRadarBatchJobs(
		context.Background(),
		"usr_owner",
		"batch_digest",
	)
	if err != nil {
		t.Fatalf("load manual Radar batch: %v", err)
	}
	if requestCount != 1 || len(jobs) != 1 {
		t.Fatalf("requests = %d, jobs = %d", requestCount, len(jobs))
	}
	if jobs[0].Job.ID != "job_radar_1" || jobs[0].Job.State != "completed" ||
		jobs[0].CandidateState != "delivered" || jobs[0].DeliveryState != "delivered" {
		t.Fatalf("unexpected batch job: %#v", jobs[0])
	}
}

func tursoManualBatchResult(requestNumber int) string {
	switch requestNumber {
	case 2:
		return `{"baton":"tx-1","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"id"}],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
	case 3:
		return `{"baton":"tx-1","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"radar_items_per_day"}],"rows":[[{"type":"integer","value":"3"}]],"affected_row_count":0,"last_insert_rowid":null}}}]}`
	case 4:
		return `{"baton":"tx-1","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"id"},{"name":"source_id"},{"name":"normalized_url"},{"name":"relevance_score"}],"rows":[[{"type":"text","value":"rad_first"},{"type":"text","value":"src_one"},{"type":"text","value":"https://openai.com/news/update"},{"type":"float","value":0.99}],[{"type":"text","value":"rad_second"},{"type":"text","value":"src_two"},{"type":"text","value":"https://go.dev/blog/update"},{"type":"float","value":0.97}],[{"type":"text","value":"rad_third"},{"type":"text","value":"src_three"},{"type":"text","value":"https://github.blog/update"},{"type":"float","value":0.95}]],"affected_row_count":0,"last_insert_rowid":null}}}]}`
	case 5, 6:
		return `{"baton":"tx-1","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":3,"last_insert_rowid":null}}}]}`
	case 7:
		return `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
	default:
		return `{"baton":"tx-1","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
	}
}
