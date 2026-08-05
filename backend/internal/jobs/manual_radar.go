package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"coreloop/backend/internal/securehash"
	"coreloop/backend/internal/store"
)

const manualRadarKeyPrefix = "manual-radar:"

var (
	ErrManualRadarNotFound    = errors.New("manual Radar batch not found")
	ErrManualRadarUnavailable = errors.New("no eligible Radar item is ready")
)

type ManualRadar struct {
	BatchID        string `json:"batch_id"`
	State          string `json:"state"`
	ProfileTarget  int    `json:"profile_target"`
	RequestedCount int    `json:"requested_count"`
	SelectedCount  int    `json:"selected_count"`
	DeliveredCount int    `json:"delivered_count"`
	FailedCount    int    `json:"failed_count"`
	Message        string `json:"message"`
}

type radarJobMetadata struct {
	CandidateID    string `json:"candidate_id"`
	ManualBatchID  string `json:"manual_batch_id,omitempty"`
	ProfileTarget  int    `json:"profile_target,omitempty"`
	RequestedCount int    `json:"requested_count,omitempty"`
}

type manualRadarStore interface {
	EnqueueManualRadarBatch(
		context.Context,
		string,
		string,
		string,
		time.Time,
	) ([]store.ManualRadarBatchJob, error)
	ManualRadarBatchJobs(context.Context, string, string) ([]store.ManualRadarBatchJob, error)
}

func (service *Service) TriggerRadarNow(
	ctx context.Context,
	userID string,
	requestID string,
) (ManualRadar, error) {
	return triggerRadarNow(
		ctx,
		service.store,
		service.publisher,
		service.appOrigin,
		service.now(),
		userID,
		requestID,
	)
}

func triggerRadarNow(
	ctx context.Context,
	dataStore manualRadarStore,
	publisher manualLessonPublisher,
	appOrigin string,
	now time.Time,
	userID string,
	requestID string,
) (ManualRadar, error) {
	batchID := manualRadarBatchID(userID, requestID)
	batchJobs, err := dataStore.EnqueueManualRadarBatch(
		ctx,
		userID,
		batchID,
		manualRadarBatchPrefix(userID, batchID),
		now,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ManualRadar{}, ErrManualRadarUnavailable
	}
	if err != nil {
		return ManualRadar{}, err
	}
	status, err := manualRadarStatus(userID, batchID, batchJobs)
	if err != nil {
		return ManualRadar{}, err
	}
	slog.InfoContext(
		ctx,
		"manual Radar durable batch ready",
		"batch_id", batchID,
		"selected_count", len(batchJobs),
		"profile_target", status.ProfileTarget,
	)
	firstJob, ready := firstQueuedManualRadarJob(batchJobs)
	if !ready || status.State != "queued" {
		return status, nil
	}
	if err := publishJob(
		ctx,
		publisher,
		appOrigin,
		firstJob.ID,
		firstJob.AttemptCount,
	); err != nil {
		slog.WarnContext(
			ctx,
			"immediate Radar batch dispatch failed",
			"batch_id", batchID,
			"job_id", firstJob.ID,
			"error", err,
		)
		status.Message += " Immediate dispatch is unavailable, so the scheduler will retry it."
	} else {
		slog.InfoContext(
			ctx,
			"immediate Radar batch dispatch accepted",
			"batch_id", batchID,
			"job_id", firstJob.ID,
			"attempt", firstJob.AttemptCount,
		)
	}
	return status, nil
}

func firstQueuedManualRadarJob(
	batchJobs []store.ManualRadarBatchJob,
) (store.Job, bool) {
	for _, batchJob := range batchJobs {
		if batchJob.Job.State == "queued" {
			return batchJob.Job, true
		}
	}
	return store.Job{}, false
}

func (service *Service) ManualRadarStatus(
	ctx context.Context,
	userID string,
	batchID string,
) (ManualRadar, error) {
	batchJobs, err := service.store.ManualRadarBatchJobs(ctx, userID, batchID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ManualRadar{}, ErrManualRadarNotFound
		}
		return ManualRadar{}, err
	}
	return manualRadarStatus(userID, batchID, batchJobs)
}

func manualRadarStatus(
	userID string,
	batchID string,
	batchJobs []store.ManualRadarBatchJob,
) (ManualRadar, error) {
	if len(batchJobs) == 0 {
		return ManualRadar{}, ErrManualRadarNotFound
	}

	metadata, valid := radarMetadata(batchJobs[0].Job)
	if !valid || metadata.ManualBatchID != batchID {
		return ManualRadar{}, ErrManualRadarNotFound
	}
	status := ManualRadar{
		BatchID:        batchID,
		ProfileTarget:  metadata.ProfileTarget,
		RequestedCount: metadata.RequestedCount,
		SelectedCount:  len(batchJobs),
	}
	active := false
	started := false
	for _, batchJob := range batchJobs {
		job := batchJob.Job
		jobMetadata, valid := radarMetadata(job)
		if job.UserID != userID || job.Type != "deliver_radar" || !valid ||
			jobMetadata.ManualBatchID != batchID ||
			!strings.HasPrefix(job.IdempotencyKey, manualRadarBatchPrefix(userID, batchID)) {
			return ManualRadar{}, ErrManualRadarNotFound
		}
		switch job.State {
		case "queued":
			active = true
		case "leased":
			active = true
			started = true
		case "failed", "cancelled", "blocked_quota":
			status.FailedCount++
		case "completed":
			started = true
			delivered, failed := manualRadarDeliveryOutcome(
				jobMetadata.CandidateID,
				batchJob.CandidateState,
				batchJob.DeliveryState,
			)
			if delivered {
				status.DeliveredCount++
			} else if failed {
				status.FailedCount++
			} else {
				active = true
			}
		default:
			status.FailedCount++
		}
	}

	finished := status.DeliveredCount+status.FailedCount == status.SelectedCount
	switch {
	case finished && status.FailedCount > 0:
		status.State = "failed"
		status.Message = fmt.Sprintf(
			"Delivered %d of %d Radar updates; %d could not be delivered.",
			status.DeliveredCount,
			status.SelectedCount,
			status.FailedCount,
		)
	case finished:
		status.State = "delivered"
		status.Message = fmt.Sprintf(
			"Delivered %d Radar updates, each as one sourced Telegram message.",
			status.DeliveredCount,
		)
	case started || status.DeliveredCount > 0:
		status.State = "delivering"
		status.Message = fmt.Sprintf(
			"Delivered %d of %d selected Radar updates.",
			status.DeliveredCount,
			status.SelectedCount,
		)
	case active:
		status.State = "queued"
		status.Message = manualRadarQueuedMessage(status)
	default:
		status.State = "failed"
		status.Message = "The Radar batch entered an unknown state."
	}
	return status, nil
}

func manualRadarDeliveryOutcome(
	candidateID string,
	candidateState string,
	deliveryState string,
) (bool, bool) {
	if candidateID == "" || candidateState == "" {
		return false, true
	}
	if candidateState == "delivered" || candidateState == "skipped" ||
		deliveryState == "delivered" {
		return true, false
	}
	if candidateState == "rejected" || deliveryState == "failed" ||
		deliveryState == "partial" {
		return false, true
	}
	return false, false
}

func manualRadarQueuedMessage(status ManualRadar) string {
	if status.ProfileTarget == 0 {
		return fmt.Sprintf(
			"Your saved Radar target is unlimited. Queued %d eligible updates for this bounded manual batch.",
			status.SelectedCount,
		)
	}
	if status.SelectedCount < status.RequestedCount {
		return fmt.Sprintf(
			"Queued %d of your saved target of %d; only %d eligible updates are ready.",
			status.SelectedCount,
			status.ProfileTarget,
			status.SelectedCount,
		)
	}
	return fmt.Sprintf(
		"Queued %d Radar updates from your saved profile target.",
		status.SelectedCount,
	)
}

func radarMetadata(job store.Job) (radarJobMetadata, bool) {
	var metadata radarJobMetadata
	if err := json.Unmarshal([]byte(job.PayloadJSON), &metadata); err != nil {
		return radarJobMetadata{}, false
	}
	return metadata, metadata.CandidateID != ""
}

func radarCandidateID(job store.Job) string {
	metadata, _ := radarMetadata(job)
	return metadata.CandidateID
}

func manualRadarBatchID(userID, requestID string) string {
	input := userID + ":" + strings.TrimSpace(requestID)
	return securehash.SHA256(input)
}

func manualRadarBatchPrefix(userID, batchID string) string {
	return manualRadarPrefix(userID) + batchID + ":"
}

func manualRadarPrefix(userID string) string {
	return manualRadarKeyPrefix + userID + ":"
}
