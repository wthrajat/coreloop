package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/store"
)

type fakeManualLessonStore struct {
	job             store.Job
	assignmentState string
	deliveryState   string
	enqueuedPayload any
}

func (fake *fakeManualLessonStore) EnqueueJob(
	_ context.Context,
	userID string,
	assignmentID string,
	jobType string,
	dueAt time.Time,
	idempotencyKey string,
	payload any,
) (string, error) {
	if fake.job.IdempotencyKey == idempotencyKey {
		return fake.job.ID, nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	fake.enqueuedPayload = payload
	fake.job = store.Job{
		ID:             "job_manual",
		UserID:         userID,
		AssignmentID:   assignmentID,
		Type:           jobType,
		State:          "queued",
		DueAt:          dueAt,
		IdempotencyKey: idempotencyKey,
		PayloadJSON:    string(encoded),
	}
	return fake.job.ID, nil
}

func TestTriggerLessonNowIsIdempotentForARepeatedRequest(t *testing.T) {
	dataStore := &fakeManualLessonStore{}
	publisher := &fakeManualLessonPublisher{}
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)

	first, err := triggerLessonNow(
		context.Background(),
		dataStore,
		publisher,
		"https://coreloop.example",
		now,
		"usr_owner",
		"same-request",
	)
	if err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	second, err := triggerLessonNow(
		context.Background(),
		dataStore,
		publisher,
		"https://coreloop.example",
		now.Add(time.Minute),
		"usr_owner",
		"same-request",
	)
	if err != nil {
		t.Fatalf("second trigger: %v", err)
	}
	if first.JobID != second.JobID {
		t.Fatalf("expected one durable job, got %q and %q", first.JobID, second.JobID)
	}
	if publisher.deduplicationID != "dispatch:"+first.JobID {
		t.Fatalf("unexpected publish deduplication id: %q", publisher.deduplicationID)
	}
}

func (fake *fakeManualLessonStore) Job(_ context.Context, jobID string) (store.Job, error) {
	if fake.job.ID != jobID {
		return store.Job{}, errors.New("job not found")
	}
	return fake.job, nil
}

func (fake *fakeManualLessonStore) AssignmentDeliveryState(_ context.Context, userID, assignmentID string) (string, string, error) {
	if fake.job.UserID != userID || fake.job.AssignmentID != assignmentID {
		return "", "", errors.New("assignment not found")
	}
	return fake.assignmentState, fake.deliveryState, nil
}

type fakeManualLessonPublisher struct {
	destination     string
	deduplicationID string
	payload         any
	err             error
}

func (fake *fakeManualLessonPublisher) Publish(_ context.Context, destination, deduplicationID string, payload any) error {
	fake.destination = destination
	fake.deduplicationID = deduplicationID
	fake.payload = payload
	return fake.err
}

func TestTriggerLessonNowQueuesAndPublishes(t *testing.T) {
	dataStore := &fakeManualLessonStore{}
	publisher := &fakeManualLessonPublisher{}
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)

	status, err := triggerLessonNow(
		context.Background(),
		dataStore,
		publisher,
		"https://coreloop.example/",
		now,
		"usr_owner",
		"request-123",
	)
	if err != nil {
		t.Fatalf("trigger lesson: %v", err)
	}
	if status.JobID != "job_manual" || status.State != "queued" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if dataStore.job.Type != "generate_lesson" || !dataStore.job.DueAt.Equal(now) {
		t.Fatalf("unexpected queued job: %#v", dataStore.job)
	}
	if !strings.HasPrefix(dataStore.job.IdempotencyKey, manualLessonPrefix("usr_owner")) {
		t.Fatalf("unexpected idempotency key: %q", dataStore.job.IdempotencyKey)
	}
	if strings.Contains(dataStore.job.IdempotencyKey, "request-123") {
		t.Fatal("raw request id must not be stored in the idempotency key")
	}
	metadata, ok := dataStore.enqueuedPayload.(lessonJobMetadata)
	if !ok || metadata.RequestKind != "manual_owner" || metadata.DispatchDelivery != "immediate" {
		t.Fatalf("unexpected job metadata: %#v", dataStore.enqueuedPayload)
	}
	if publisher.destination != "https://coreloop.example/api/jobs/run" || publisher.deduplicationID != "dispatch:job_manual" {
		t.Fatalf("unexpected publish call: %#v", publisher)
	}
}

func TestTriggerLessonNowKeepsDurableJobWhenImmediatePublishFails(t *testing.T) {
	dataStore := &fakeManualLessonStore{}
	publisher := &fakeManualLessonPublisher{err: errors.New("QStash unavailable")}

	status, err := triggerLessonNow(
		context.Background(),
		dataStore,
		publisher,
		"https://coreloop.example",
		time.Now(),
		"usr_owner",
		"request-456",
	)
	if err != nil {
		t.Fatalf("trigger lesson: %v", err)
	}
	if status.State != "queued" || !strings.Contains(status.Message, "scheduler will retry") {
		t.Fatalf("unexpected fallback status: %#v", status)
	}
}

func TestManualLessonStatusTracksGenerationAndDelivery(t *testing.T) {
	dataStore := &fakeManualLessonStore{}
	baseJob := store.Job{
		ID:             "job_manual",
		UserID:         "usr_owner",
		Type:           "generate_lesson",
		IdempotencyKey: manualLessonPrefix("usr_owner") + "digest",
	}

	for _, test := range []struct {
		jobState string
		want     string
	}{
		{jobState: "queued", want: "queued"},
		{jobState: "leased", want: "generating"},
		{jobState: "blocked_quota", want: "quota_blocked"},
		{jobState: "failed", want: "failed"},
	} {
		job := baseJob
		job.State = test.jobState
		status, err := manualLessonStatus(context.Background(), dataStore, "usr_owner", job)
		if err != nil {
			t.Fatalf("status for %s: %v", test.jobState, err)
		}
		if status.State != test.want {
			t.Fatalf("state %s: expected %s, got %#v", test.jobState, test.want, status)
		}
	}

	deliveredJob := baseJob
	deliveredJob.State = "completed"
	deliveredJob.AssignmentID = "asn_test"
	dataStore.job = deliveredJob
	dataStore.assignmentState = "delivered"
	dataStore.deliveryState = "completed"
	status, err := manualLessonStatus(context.Background(), dataStore, "usr_owner", deliveredJob)
	if err != nil {
		t.Fatalf("delivered status: %v", err)
	}
	if status.State != "delivered" || !strings.Contains(status.Message, "Read and Skip") {
		t.Fatalf("unexpected delivered status: %#v", status)
	}

	_, err = manualLessonStatus(context.Background(), dataStore, "usr_friend", deliveredJob)
	if !errors.Is(err, ErrManualLessonNotFound) {
		t.Fatalf("expected private not-found result, got %v", err)
	}
}

func TestManualLessonMetadataControlsProviderRecordAndDeliveryDispatch(t *testing.T) {
	job := store.Job{PayloadJSON: `{"request_kind":"manual_owner","dispatch_delivery":"immediate"}`}
	if requestKind(job, false) != "manual_owner" {
		t.Fatal("manual generation must be recorded as an owner request")
	}
	if !dispatchLessonDeliveryImmediately(job) {
		t.Fatal("manual generation must dispatch its delivery immediately")
	}
	if requestKind(store.Job{}, false) != "scheduled" {
		t.Fatal("ordinary lesson jobs must remain scheduled")
	}
}
