package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"coreloop/backend/internal/securehash"
	"coreloop/backend/internal/store"
)

const manualRadarKeyPrefix = "manual-radar:"

var (
	ErrManualRadarNotFound    = errors.New("manual Radar job not found")
	ErrManualRadarUnavailable = errors.New("no eligible Radar item is ready")
)

type ManualRadar struct {
	JobID   string `json:"job_id"`
	State   string `json:"state"`
	Message string `json:"message"`
}

type manualRadarStore interface {
	EnqueueManualRadar(context.Context, string, string, time.Time) (string, error)
	Job(context.Context, string) (store.Job, error)
	RadarCandidateDeliveryState(context.Context, string, string) (string, string, error)
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
	jobID, err := dataStore.EnqueueManualRadar(
		ctx,
		userID,
		manualRadarKey(userID, requestID),
		now,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ManualRadar{}, ErrManualRadarUnavailable
	}
	if err != nil {
		return ManualRadar{}, err
	}
	job, err := dataStore.Job(ctx, jobID)
	if err != nil {
		return ManualRadar{}, err
	}
	status, err := manualRadarStatus(ctx, dataStore, userID, job)
	if err != nil {
		return ManualRadar{}, err
	}
	slog.InfoContext(
		ctx,
		"manual Radar durable job ready",
		"job_id", job.ID,
		"job_state", job.State,
		"attempt", job.AttemptCount,
	)
	if job.State != "queued" {
		return status, nil
	}
	if err := publishJob(ctx, publisher, appOrigin, job.ID, job.AttemptCount); err != nil {
		slog.WarnContext(ctx, "immediate Radar dispatch failed", "job_id", job.ID, "error", err)
		status.Message = "Radar queued. Immediate dispatch is unavailable, so the scheduler will retry it."
	} else {
		slog.InfoContext(ctx, "immediate Radar dispatch accepted", "job_id", job.ID, "attempt", job.AttemptCount)
	}
	return status, nil
}

func (service *Service) ManualRadarStatus(
	ctx context.Context,
	userID string,
	jobID string,
) (ManualRadar, error) {
	job, err := service.store.Job(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ManualRadar{}, ErrManualRadarNotFound
		}
		return ManualRadar{}, err
	}
	return manualRadarStatus(ctx, service.store, userID, job)
}

func manualRadarStatus(
	ctx context.Context,
	dataStore manualRadarStore,
	userID string,
	job store.Job,
) (ManualRadar, error) {
	if job.UserID != userID || job.Type != "deliver_radar" ||
		!strings.HasPrefix(job.IdempotencyKey, manualRadarPrefix(userID)) {
		return ManualRadar{}, ErrManualRadarNotFound
	}
	status := ManualRadar{JobID: job.ID}
	switch job.State {
	case "queued":
		status.State = "queued"
		status.Message = "The best eligible Radar update is queued for Telegram."
	case "leased":
		status.State = "delivering"
		status.Message = "Sending the selected Radar update to Telegram."
	case "failed", "cancelled", "blocked_quota":
		status.State = "failed"
		status.Message = "Radar delivery failed. Review the queue state and try again."
	case "completed":
		candidateID := radarCandidateID(job)
		if candidateID == "" {
			status.State = "failed"
			status.Message = "Radar delivery completed without a selected update."
			return status, nil
		}
		candidateState, deliveryState, err := dataStore.RadarCandidateDeliveryState(
			ctx,
			userID,
			candidateID,
		)
		if err != nil {
			return ManualRadar{}, err
		}
		switch {
		case candidateState == "delivered" || candidateState == "skipped" || deliveryState == "delivered":
			status.State = "delivered"
			status.Message = "Delivered as one sourced Radar message in Telegram."
		case candidateState == "rejected" || deliveryState == "failed" || deliveryState == "partial":
			status.State = "failed"
			status.Message = "The selected update could not be delivered to Telegram."
		default:
			status.State = "delivering"
			status.Message = "The selected Radar update is still being delivered."
		}
	default:
		status.State = "failed"
		status.Message = "The Radar job entered an unknown state."
	}
	return status, nil
}

func radarCandidateID(job store.Job) string {
	var payload struct {
		CandidateID string `json:"candidate_id"`
	}
	_ = json.Unmarshal([]byte(job.PayloadJSON), &payload)
	return payload.CandidateID
}

func manualRadarKey(userID, requestID string) string {
	return manualRadarPrefix(userID) + securehash.SHA256(strings.TrimSpace(requestID))
}

func manualRadarPrefix(userID string) string {
	return manualRadarKeyPrefix + userID + ":"
}
