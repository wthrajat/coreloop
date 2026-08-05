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

func TestEnqueueManualRadarUsesOneTransactionWithoutDailyUsage(t *testing.T) {
	requestCount := 0
	var requests []string
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		requestCount++
		body, _ := io.ReadAll(request.Body)
		requests = append(requests, string(body))

		responseBody := tursoManualResult(requestCount)
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
	jobID, err := dataStore.EnqueueManualRadar(
		context.Background(),
		"usr_owner",
		"manual-radar:usr_owner:digest",
		time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("enqueue manual Radar: %v", err)
	}
	if !strings.HasPrefix(jobID, "job_") {
		t.Fatalf("job id = %q", jobID)
	}
	if requestCount != 6 {
		t.Fatalf("request count = %d, want 6", requestCount)
	}

	combined := []byte(strings.Join(requests, "\n"))
	for _, required := range [][]byte{
		[]byte(`"sql":"BEGIN"`),
		[]byte(`SELECT id FROM job_queue WHERE idempotency_key=?`),
		[]byte(`rc.relevance_score\u003e=?`),
		[]byte(`SET status='qualified',updated_at=?`),
		[]byte(`"value":"deliver_radar"`),
		[]byte(`"value":"rad_best"`),
		[]byte(`"sql":"COMMIT"`),
	} {
		if !bytes.Contains(combined, required) {
			t.Fatalf("missing request fragment %q in %s", required, combined)
		}
	}
	if bytes.Contains(combined, []byte("radar_daily_usage")) ||
		bytes.Contains(combined, []byte("released_at")) {
		t.Fatalf("manual send consumed normal Radar cadence: %s", combined)
	}
}

func tursoManualResult(requestNumber int) string {
	switch requestNumber {
	case 2:
		return `{"baton":"tx-1","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"id"}],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
	case 3:
		return `{"baton":"tx-1","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"id"},{"name":"normalized_url"}],"rows":[[{"type":"text","value":"rad_best"},{"type":"text","value":"https://example.co/update"}]],"affected_row_count":0,"last_insert_rowid":null}}}]}`
	case 4, 5:
		return `{"baton":"tx-1","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":1,"last_insert_rowid":null}}}]}`
	case 6:
		return `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
	default:
		return `{"baton":"tx-1","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}}]}`
	}
}
