package store

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"coreloop/backend/internal/database/tursohttp"
)

func TestEnqueueSourcePollsDoesNotDuplicateActivePollJobs(t *testing.T) {
	var requestBody []byte
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		requestBody, _ = io.ReadAll(request.Body)
		return tursoTestResponse(
			`{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[{"name":"id"},{"name":"polling_interval_minutes"},{"name":"last_polled_at"}],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`,
		), nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := New(database).EnqueueSourcePolls(
		context.Background(),
		time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatal(err)
	}
	for _, required := range [][]byte{
		[]byte("job_type='ingest_source'"),
		[]byte("state IN ('queued','leased')"),
		[]byte("json_extract(jq.payload_json,'$.source_id')=s.id"),
	} {
		if !bytes.Contains(requestBody, required) {
			t.Fatalf("source poll query omitted %q: %s", required, requestBody)
		}
	}
}
