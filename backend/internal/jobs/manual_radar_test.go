package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/store"
)

type fakeManualRadarStore struct {
	job            store.Job
	candidateState string
	deliveryState  string
	enqueueError   error
}

func (fake *fakeManualRadarStore) EnqueueManualRadar(
	_ context.Context,
	userID string,
	idempotencyKey string,
	dueAt time.Time,
) (string, error) {
	if fake.enqueueError != nil {
		return "", fake.enqueueError
	}
	if fake.job.IdempotencyKey == idempotencyKey {
		return fake.job.ID, nil
	}
	payload, err := json.Marshal(map[string]string{"candidate_id": "rad_best"})
	if err != nil {
		return "", err
	}
	fake.job = store.Job{
		ID:             "job_radar",
		UserID:         userID,
		Type:           "deliver_radar",
		State:          "queued",
		DueAt:          dueAt,
		IdempotencyKey: idempotencyKey,
		PayloadJSON:    string(payload),
	}
	return fake.job.ID, nil
}

func (fake *fakeManualRadarStore) Job(_ context.Context, jobID string) (store.Job, error) {
	if fake.job.ID != jobID {
		return store.Job{}, sql.ErrNoRows
	}
	return fake.job, nil
}

func (fake *fakeManualRadarStore) RadarCandidateDeliveryState(
	_ context.Context,
	userID string,
	candidateID string,
) (string, string, error) {
	if fake.job.UserID != userID || radarCandidateID(fake.job) != candidateID {
		return "", "", sql.ErrNoRows
	}
	return fake.candidateState, fake.deliveryState, nil
}

func TestTriggerRadarNowQueuesAndPublishes(t *testing.T) {
	dataStore := &fakeManualRadarStore{}
	publisher := &fakeManualLessonPublisher{}
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)

	status, err := triggerRadarNow(
		context.Background(),
		dataStore,
		publisher,
		"https://coreloop.example/",
		now,
		"usr_owner",
		"request-123",
	)
	if err != nil {
		t.Fatalf("trigger Radar: %v", err)
	}
	if status.JobID != "job_radar" || status.State != "queued" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if dataStore.job.Type != "deliver_radar" || !dataStore.job.DueAt.Equal(now) {
		t.Fatalf("unexpected queued job: %#v", dataStore.job)
	}
	if !strings.HasPrefix(dataStore.job.IdempotencyKey, manualRadarPrefix("usr_owner")) {
		t.Fatalf("unexpected idempotency key: %q", dataStore.job.IdempotencyKey)
	}
	if strings.Contains(dataStore.job.IdempotencyKey, "request-123") {
		t.Fatal("raw request id must not be stored in the idempotency key")
	}
	if publisher.destination != "https://coreloop.example/api/jobs/run" ||
		publisher.deduplicationID != "dispatch-job_radar-0" {
		t.Fatalf("unexpected publish call: %#v", publisher)
	}
}

func TestTriggerRadarNowKeepsDurableJobWhenImmediatePublishFails(t *testing.T) {
	dataStore := &fakeManualRadarStore{}
	publisher := &fakeManualLessonPublisher{err: errors.New("QStash unavailable")}

	status, err := triggerRadarNow(
		context.Background(),
		dataStore,
		publisher,
		"https://coreloop.example",
		time.Now(),
		"usr_owner",
		"request-456",
	)
	if err != nil {
		t.Fatalf("trigger Radar: %v", err)
	}
	if status.State != "queued" || !strings.Contains(status.Message, "scheduler will retry") {
		t.Fatalf("unexpected fallback status: %#v", status)
	}
}

func TestTriggerRadarNowReportsWhenNothingEligibleIsReady(t *testing.T) {
	dataStore := &fakeManualRadarStore{enqueueError: sql.ErrNoRows}

	_, err := triggerRadarNow(
		context.Background(),
		dataStore,
		&fakeManualLessonPublisher{},
		"https://coreloop.example",
		time.Now(),
		"usr_owner",
		"request-789",
	)
	if !errors.Is(err, ErrManualRadarUnavailable) {
		t.Fatalf("expected unavailable result, got %v", err)
	}
}

func TestManualRadarStatusTracksDeliveryAndProtectsOwnership(t *testing.T) {
	dataStore := &fakeManualRadarStore{}
	_, err := dataStore.EnqueueManualRadar(
		context.Background(),
		"usr_owner",
		manualRadarPrefix("usr_owner")+"digest",
		time.Now(),
	)
	if err != nil {
		t.Fatalf("prepare fake job: %v", err)
	}

	for _, test := range []struct {
		jobState string
		want     string
	}{
		{jobState: "queued", want: "queued"},
		{jobState: "leased", want: "delivering"},
		{jobState: "failed", want: "failed"},
	} {
		job := dataStore.job
		job.State = test.jobState
		status, statusErr := manualRadarStatus(context.Background(), dataStore, "usr_owner", job)
		if statusErr != nil {
			t.Fatalf("status for %s: %v", test.jobState, statusErr)
		}
		if status.State != test.want {
			t.Fatalf("state %s: expected %s, got %#v", test.jobState, test.want, status)
		}
	}

	completedJob := dataStore.job
	completedJob.State = "completed"
	dataStore.job = completedJob
	dataStore.candidateState = "delivered"
	dataStore.deliveryState = "delivered"
	status, err := manualRadarStatus(context.Background(), dataStore, "usr_owner", completedJob)
	if err != nil {
		t.Fatalf("delivered status: %v", err)
	}
	if status.State != "delivered" || !strings.Contains(status.Message, "one sourced") {
		t.Fatalf("unexpected delivered status: %#v", status)
	}

	_, err = manualRadarStatus(context.Background(), dataStore, "usr_friend", completedJob)
	if !errors.Is(err, ErrManualRadarNotFound) {
		t.Fatalf("expected private not-found result, got %v", err)
	}
}
