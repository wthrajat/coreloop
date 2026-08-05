package store

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/database/tursohttp"
)

func TestUpdatePreferencesUsesBoundedDatabaseRequests(t *testing.T) {
	requestCount := 0
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		requestCount++
		requestBody, _ := io.ReadAll(request.Body)
		body := `{"baton":"preferences-tx","results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":2,"last_insert_rowid":null}}}]}`
		if bytes.Contains(requestBody, []byte(`"sql":"COMMIT"`)) {
			body = `{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`
		}
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

	err = New(database).UpdatePreferences(context.Background(), "user", Preferences{
		LessonMinutes: 15, ExplanationDepth: "standard", LessonsPerDay: 3,
		RadarEnabled: true, RadarItemsPerDay: 8, RadarWeekendsEnabled: true,
		RecallMode: "off", BundleMode: "complete", TimeZone: "Asia/Kolkata",
		DeliveryTimes: []string{"08:30", "13:00", "20:30"},
		TopicIDs:      []string{"topic_one", "topic_two"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if requestCount != 7 {
		t.Fatalf("database requests = %d, want fixed budget of 7", requestCount)
	}
}

func TestCompleteJobRejectsLostLease(t *testing.T) {
	client := &http.Client{Transport: radarManualRoundTripFunc(func(
		*http.Request,
	) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"results":[{"type":"ok","response":{"type":"execute","result":{"cols":[],"rows":[],"affected_row_count":0,"last_insert_rowid":null}}},{"type":"ok","response":{"type":"close"}}]}`,
			)),
			Header: make(http.Header),
		}, nil
	})}
	database, err := tursohttp.Open("libsql://example.turso.io", "secret", client)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	err = New(database).CompleteJob(
		context.Background(), "job", "stale-worker", time.Now(),
	)
	if !errors.Is(err, ErrJobLeaseLost) {
		t.Fatalf("completion error = %v, want ErrJobLeaseLost", err)
	}
}
