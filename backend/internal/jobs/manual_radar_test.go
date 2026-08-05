package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"coreloop/backend/internal/store"
)

type fakeManualRadarStore struct {
	jobs          []store.ManualRadarBatchJob
	profileTarget int
	selectedCount int
	enqueueError  error
}

func (fake *fakeManualRadarStore) EnqueueManualRadarBatch(
	_ context.Context,
	userID string,
	batchID string,
	idempotencyPrefix string,
	dueAt time.Time,
) ([]store.ManualRadarBatchJob, error) {
	if fake.enqueueError != nil {
		return nil, fake.enqueueError
	}
	if len(fake.jobs) > 0 {
		metadata, _ := radarMetadata(fake.jobs[0].Job)
		if metadata.ManualBatchID == batchID {
			return fake.jobs, nil
		}
	}
	target := fake.profileTarget
	if target == 0 {
		target = 20
	}
	selected := fake.selectedCount
	if selected == 0 {
		selected = target
	}
	for index := range selected {
		candidateID := fmt.Sprintf("rad_%d", index+1)
		payload, err := json.Marshal(radarJobMetadata{
			CandidateID:    candidateID,
			ManualBatchID:  batchID,
			ProfileTarget:  fake.profileTarget,
			RequestedCount: target,
		})
		if err != nil {
			return nil, err
		}
		fake.jobs = append(fake.jobs, store.ManualRadarBatchJob{
			Job: store.Job{
				ID:             fmt.Sprintf("job_radar_%d", index+1),
				UserID:         userID,
				Type:           "deliver_radar",
				State:          "queued",
				DueAt:          dueAt,
				IdempotencyKey: fmt.Sprintf("%s%d", idempotencyPrefix, index+1),
				PayloadJSON:    string(payload),
			},
			CandidateState: "qualified",
		})
	}
	return fake.jobs, nil
}

func (fake *fakeManualRadarStore) ManualRadarBatchJobs(
	_ context.Context,
	userID string,
	batchID string,
) ([]store.ManualRadarBatchJob, error) {
	if len(fake.jobs) == 0 || fake.jobs[0].Job.UserID != userID {
		return nil, sql.ErrNoRows
	}
	metadata, valid := radarMetadata(fake.jobs[0].Job)
	if !valid || metadata.ManualBatchID != batchID {
		return nil, sql.ErrNoRows
	}
	return fake.jobs, nil
}

func TestTriggerRadarNowQueuesSavedProfileTargetAndPublishesFirstJob(t *testing.T) {
	dataStore := &fakeManualRadarStore{profileTarget: 3}
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
	if status.State != "queued" || status.ProfileTarget != 3 ||
		status.RequestedCount != 3 || status.SelectedCount != 3 {
		t.Fatalf("unexpected status: %#v", status)
	}
	if status.BatchID == "" || strings.Contains(status.BatchID, "request-123") {
		t.Fatalf("unsafe batch id: %q", status.BatchID)
	}
	if len(dataStore.jobs) != 3 {
		t.Fatalf("queued jobs = %d, want 3", len(dataStore.jobs))
	}
	for index, batchJob := range dataStore.jobs {
		job := batchJob.Job
		if job.Type != "deliver_radar" || !job.DueAt.Equal(now) {
			t.Fatalf("unexpected queued job: %#v", job)
		}
		wantPrefix := manualRadarBatchPrefix("usr_owner", status.BatchID)
		if !strings.HasPrefix(job.IdempotencyKey, wantPrefix) {
			t.Fatalf("job %d idempotency key = %q", index, job.IdempotencyKey)
		}
	}
	if publisher.destination != "https://coreloop.example/api/jobs/run" ||
		publisher.deduplicationID != "dispatch-job_radar_1-0" {
		t.Fatalf("unexpected publish call: %#v", publisher)
	}
}

func TestTriggerRadarNowIsIdempotentForRepeatedRequest(t *testing.T) {
	dataStore := &fakeManualRadarStore{profileTarget: 4}
	publisher := &fakeManualLessonPublisher{}

	first, err := triggerRadarNow(
		context.Background(), dataStore, publisher, "https://coreloop.example",
		time.Now(), "usr_owner", "same-request",
	)
	if err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	second, err := triggerRadarNow(
		context.Background(), dataStore, publisher, "https://coreloop.example",
		time.Now().Add(time.Minute), "usr_owner", "same-request",
	)
	if err != nil {
		t.Fatalf("second trigger: %v", err)
	}
	if first.BatchID != second.BatchID || len(dataStore.jobs) != 4 {
		t.Fatalf("repeated request created another batch: first=%#v second=%#v jobs=%d",
			first, second, len(dataStore.jobs))
	}
}

func TestTriggerRadarNowUsesBoundedBatchForUnlimitedProfile(t *testing.T) {
	dataStore := &fakeManualRadarStore{profileTarget: 0, selectedCount: 7}

	status, err := triggerRadarNow(
		context.Background(),
		dataStore,
		&fakeManualLessonPublisher{},
		"https://coreloop.example",
		time.Now(),
		"usr_owner",
		"request-unlimited",
	)
	if err != nil {
		t.Fatalf("trigger Radar: %v", err)
	}
	if status.ProfileTarget != 0 || status.RequestedCount != 20 ||
		status.SelectedCount != 7 || !strings.Contains(status.Message, "unlimited") {
		t.Fatalf("unexpected unlimited status: %#v", status)
	}
}

func TestTriggerRadarNowKeepsDurableBatchWhenImmediatePublishFails(t *testing.T) {
	dataStore := &fakeManualRadarStore{profileTarget: 2}
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
	if len(dataStore.jobs) != 2 {
		t.Fatalf("durable jobs = %d, want 2", len(dataStore.jobs))
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

func TestManualRadarStatusAggregatesDeliveryAndProtectsOwnership(t *testing.T) {
	dataStore := &fakeManualRadarStore{profileTarget: 3}
	batchID := manualRadarBatchID("usr_owner", "status-request")
	jobs, err := dataStore.EnqueueManualRadarBatch(
		context.Background(),
		"usr_owner",
		batchID,
		manualRadarBatchPrefix("usr_owner", batchID),
		time.Now(),
	)
	if err != nil {
		t.Fatalf("prepare fake jobs: %v", err)
	}
	jobs[0].Job.State = "completed"
	jobs[0].CandidateState = "delivered"
	jobs[0].DeliveryState = "delivered"
	jobs[1].Job.State = "completed"
	jobs[1].CandidateState = "rejected"
	jobs[1].DeliveryState = "failed"
	jobs[2].Job.State = "leased"

	status, err := manualRadarStatus(
		"usr_owner",
		batchID,
		jobs,
	)
	if err != nil {
		t.Fatalf("delivery status: %v", err)
	}
	if status.State != "delivering" || status.DeliveredCount != 1 ||
		status.FailedCount != 1 || !strings.Contains(status.Message, "1 of 3") {
		t.Fatalf("unexpected aggregate status: %#v", status)
	}

	jobs[2].Job.State = "completed"
	jobs[2].CandidateState = "delivered"
	jobs[2].DeliveryState = "delivered"
	status, err = manualRadarStatus(
		"usr_owner",
		batchID,
		jobs,
	)
	if err != nil {
		t.Fatalf("finished status: %v", err)
	}
	if status.State != "failed" || status.DeliveredCount != 2 ||
		status.FailedCount != 1 || !strings.Contains(status.Message, "2 of 3") {
		t.Fatalf("unexpected finished status: %#v", status)
	}

	_, err = manualRadarStatus(
		"usr_friend",
		batchID,
		jobs,
	)
	if !errors.Is(err, ErrManualRadarNotFound) {
		t.Fatalf("expected private not-found result, got %v", err)
	}
}
