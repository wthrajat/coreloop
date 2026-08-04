package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"coreloop/backend/internal/qstash"
	"coreloop/backend/internal/securehash"
	"coreloop/backend/internal/store"
)

const manualLessonKeyPrefix = "manual-lesson:"

var ErrManualLessonNotFound = errors.New("manual lesson job not found")

type ManualLesson struct {
	JobID   string `json:"job_id"`
	State   string `json:"state"`
	Message string `json:"message"`
}

type lessonJobMetadata struct {
	RequestKind      string `json:"request_kind,omitempty"`
	DispatchDelivery string `json:"dispatch_delivery,omitempty"`
}

type manualLessonStore interface {
	EnqueueJob(context.Context, string, string, string, time.Time, string, any) (string, error)
	Job(context.Context, string) (store.Job, error)
	AssignmentDeliveryState(context.Context, string, string) (string, string, error)
}

type manualLessonPublisher interface {
	Publish(context.Context, string, string, any) error
}

func (service *Service) TriggerLessonNow(ctx context.Context, userID, requestID string) (ManualLesson, error) {
	return triggerLessonNow(
		ctx,
		service.store,
		service.publisher,
		service.appOrigin,
		service.now(),
		userID,
		requestID,
	)
}

func triggerLessonNow(
	ctx context.Context,
	dataStore manualLessonStore,
	publisher manualLessonPublisher,
	appOrigin string,
	now time.Time,
	userID string,
	requestID string,
) (ManualLesson, error) {
	idempotencyKey := manualLessonKey(userID, requestID)
	jobID, err := dataStore.EnqueueJob(
		ctx,
		userID,
		"",
		"generate_lesson",
		now,
		idempotencyKey,
		lessonJobMetadata{
			RequestKind:      "manual_owner",
			DispatchDelivery: "immediate",
		},
	)
	if err != nil {
		return ManualLesson{}, err
	}
	job, err := dataStore.Job(ctx, jobID)
	if err != nil {
		return ManualLesson{}, err
	}
	status, err := manualLessonStatus(ctx, dataStore, userID, job)
	if err != nil {
		return ManualLesson{}, err
	}
	if job.State != "queued" {
		return status, nil
	}
	if publisher == nil {
		status.Message = "Lesson queued. Immediate dispatch is unavailable, so the scheduler will retry it."
		return status, nil
	}
	if err := publishJob(ctx, publisher, appOrigin, job.ID); err != nil {
		slog.WarnContext(ctx, "immediate lesson dispatch failed", "job_id", job.ID, "error", err)
		status.Message = "Lesson queued. Immediate dispatch is unavailable, so the scheduler will retry it."
	}
	return status, nil
}

func (service *Service) ManualLessonStatus(ctx context.Context, userID, jobID string) (ManualLesson, error) {
	job, err := service.store.Job(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ManualLesson{}, ErrManualLessonNotFound
		}
		return ManualLesson{}, err
	}
	return manualLessonStatus(ctx, service.store, userID, job)
}

func manualLessonStatus(ctx context.Context, dataStore manualLessonStore, userID string, job store.Job) (ManualLesson, error) {
	if job.UserID != userID || job.Type != "generate_lesson" || !strings.HasPrefix(job.IdempotencyKey, manualLessonPrefix(userID)) {
		return ManualLesson{}, ErrManualLessonNotFound
	}
	status := ManualLesson{JobID: job.ID}
	switch job.State {
	case "queued":
		status.State = "queued"
		status.Message = "Queued for immediate generation."
	case "leased":
		status.State = "generating"
		status.Message = "Generating the next lesson from your current settings."
	case "blocked_quota":
		status.State = "quota_blocked"
		status.Message = "Free AI quota is unavailable. Use the blocked job action below or wait for an automatic retry."
	case "failed":
		status.State = "failed"
		status.Message = "Lesson generation failed. Review the failed work below and try again."
	case "completed":
		if job.AssignmentID == "" {
			status.State = "failed"
			status.Message = "Lesson generation completed without a deliverable assignment."
			return status, nil
		}
		assignmentState, deliveryState, err := dataStore.AssignmentDeliveryState(ctx, userID, job.AssignmentID)
		if err != nil {
			return ManualLesson{}, err
		}
		if deliveryState == "failed" {
			status.State = "failed"
			status.Message = "The lesson was generated, but Telegram delivery failed."
			return status, nil
		}
		switch assignmentState {
		case "delivered", "read", "skipped":
			status.State = "delivered"
			status.Message = "Delivered to Telegram with Read and Skip feedback."
		default:
			status.State = "delivering"
			status.Message = "Lesson generated. Delivering it to Telegram now."
		}
	default:
		status.State = "failed"
		status.Message = "The lesson job entered an unknown state."
	}
	return status, nil
}

func (service *Service) enqueueLessonDelivery(ctx context.Context, sourceJob store.Job, assignmentID string) error {
	if err := service.store.LinkJobAssignment(ctx, sourceJob.ID, assignmentID, service.now()); err != nil {
		return err
	}
	deliveryJobID, err := service.store.EnqueueJob(
		ctx,
		sourceJob.UserID,
		assignmentID,
		"deliver_lesson",
		service.now(),
		"deliver:"+assignmentID,
		map[string]string{"assignment_id": assignmentID},
	)
	if err != nil {
		return err
	}
	if !dispatchLessonDeliveryImmediately(sourceJob) {
		return nil
	}
	if err := publishJob(ctx, service.publisher, service.appOrigin, deliveryJobID); err != nil {
		slog.WarnContext(ctx, "immediate lesson delivery dispatch failed", "job_id", deliveryJobID, "error", err)
	}
	return nil
}

func publishJob(ctx context.Context, publisher manualLessonPublisher, appOrigin, jobID string) error {
	if publisher == nil {
		return errors.New("QStash publisher is not configured")
	}
	destination := strings.TrimRight(appOrigin, "/") + "/api/jobs/run"
	return publisher.Publish(ctx, destination, dispatchDeduplicationID(jobID), map[string]string{"job_id": jobID})
}

func manualLessonKey(userID, requestID string) string {
	return manualLessonPrefix(userID) + securehash.SHA256(strings.TrimSpace(requestID))
}

func manualLessonPrefix(userID string) string {
	return manualLessonKeyPrefix + userID + ":"
}

func lessonMetadata(job store.Job) lessonJobMetadata {
	var metadata lessonJobMetadata
	_ = json.Unmarshal([]byte(job.PayloadJSON), &metadata)
	return metadata
}

func dispatchLessonDeliveryImmediately(job store.Job) bool {
	return lessonMetadata(job).DispatchDelivery == "immediate"
}

func requestKind(job store.Job, openAI bool) string {
	if openAI || lessonMetadata(job).RequestKind == "manual_owner" {
		return "manual_owner"
	}
	return "scheduled"
}

var _ manualLessonPublisher = (*qstash.Publisher)(nil)
