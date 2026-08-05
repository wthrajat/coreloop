package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"coreloop/backend/internal/ids"
)

type manualRadarJobPayload struct {
	CandidateID    string `json:"candidate_id"`
	ManualBatchID  string `json:"manual_batch_id"`
	ProfileTarget  int    `json:"profile_target"`
	RequestedCount int    `json:"requested_count"`
}

type ManualRadarBatchJob struct {
	Job            Job
	CandidateState string
	DeliveryState  string
}

// EnqueueManualRadarBatch atomically reserves the best currently eligible
// Radar candidates up to the profile's saved target and creates one durable
// delivery job per candidate. It deliberately does not update
// radar_daily_usage or released_at, so an owner acceptance test cannot consume
// or delay normal Radar delivery.
func (store *Store) EnqueueManualRadarBatch(
	ctx context.Context,
	userID string,
	batchID string,
	idempotencyPrefix string,
	now time.Time,
) ([]ManualRadarBatchJob, error) {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	existingJobs, err := queryManualRadarBatchJobs(ctx, tx, userID, batchID)
	if err == nil {
		return existingJobs, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	profileTarget, err := manualRadarProfileTarget(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	requestedCount := profileTarget
	if requestedCount == 0 {
		requestedCount = maximumUnlimitedReleasePass
	}
	if requestedCount < 1 {
		return nil, sql.ErrNoRows
	}

	candidateIDs, err := manualRadarCandidateIDs(
		ctx,
		tx,
		userID,
		requestedCount,
		now,
	)
	if err != nil {
		return nil, err
	}
	if len(candidateIDs) == 0 {
		return nil, sql.ErrNoRows
	}

	batchJobs, err := newManualRadarJobs(
		userID,
		batchID,
		idempotencyPrefix,
		profileTarget,
		requestedCount,
		candidateIDs,
		now,
	)
	if err != nil {
		return nil, err
	}
	if err := reserveManualRadarCandidates(ctx, tx, userID, candidateIDs, now); err != nil {
		return nil, err
	}
	if err := insertManualRadarJobs(ctx, tx, batchJobs); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return batchJobs, nil
}

func newManualRadarJobs(
	userID string,
	batchID string,
	idempotencyPrefix string,
	profileTarget int,
	requestedCount int,
	candidateIDs []string,
	now time.Time,
) ([]ManualRadarBatchJob, error) {
	jobs := make([]ManualRadarBatchJob, 0, len(candidateIDs))
	for index, candidateID := range candidateIDs {
		jobID, err := ids.New("job")
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(manualRadarJobPayload{
			CandidateID:    candidateID,
			ManualBatchID:  batchID,
			ProfileTarget:  profileTarget,
			RequestedCount: requestedCount,
		})
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, ManualRadarBatchJob{
			Job: Job{
				ID:             jobID,
				UserID:         userID,
				Type:           "deliver_radar",
				State:          "queued",
				DueAt:          now,
				MaxAttempts:    5,
				IdempotencyKey: fmt.Sprintf("%s%d", idempotencyPrefix, index+1),
				PayloadJSON:    string(payload),
			},
			CandidateState: "qualified",
		})
	}
	return jobs, nil
}

func reserveManualRadarCandidates(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	candidateIDs []string,
	now time.Time,
) error {
	arguments := make([]any, 0, len(candidateIDs)+2)
	arguments = append(arguments, timestamp(now), userID)
	for _, candidateID := range candidateIDs {
		arguments = append(arguments, candidateID)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(candidateIDs)), ",")
	statement := `UPDATE radar_candidates SET status='qualified',updated_at=?
		WHERE user_id=? AND status='pending' AND id IN (` + placeholders + `)`
	result, err := tx.ExecContext(ctx, statement, arguments...)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != int64(len(candidateIDs)) {
		return fmt.Errorf("reserve manual Radar batch: %w", sql.ErrNoRows)
	}
	return nil
}

func insertManualRadarJobs(
	ctx context.Context,
	tx *sql.Tx,
	jobs []ManualRadarBatchJob,
) error {
	values := strings.TrimSuffix(strings.Repeat("(?,?,?,?,?,?),", len(jobs)), ",")
	arguments := make([]any, 0, len(jobs)*6)
	for _, batchJob := range jobs {
		job := batchJob.Job
		arguments = append(
			arguments,
			job.ID,
			job.UserID,
			job.Type,
			timestamp(job.DueAt),
			job.IdempotencyKey,
			job.PayloadJSON,
		)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO job_queue
		(id,user_id,job_type,due_at,idempotency_key,payload_json) VALUES `+values,
		arguments...)
	return err
}

func manualRadarProfileTarget(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
) (int, error) {
	var target int
	err := tx.QueryRowContext(ctx, `SELECT lp.radar_items_per_day
		FROM learning_preferences lp
		JOIN users u ON u.id=lp.user_id AND u.status='active'
		JOIN delivery_destinations dd ON dd.user_id=lp.user_id
			AND dd.channel='telegram' AND dd.enabled=1 AND dd.status='connected'
		WHERE lp.user_id=? AND lp.radar_enabled=1`, userID).Scan(&target)
	return target, err
}

func manualRadarCandidateIDs(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	limit int,
	now time.Time,
) ([]string, error) {
	pool, err := radarCandidatePool(ctx, tx, userID, limit, now)
	if err != nil {
		return nil, err
	}
	return diverseRadarCandidates(pool, nil, limit), nil
}

func (store *Store) ManualRadarBatchJobs(
	ctx context.Context,
	userID string,
	batchID string,
) ([]ManualRadarBatchJob, error) {
	return queryManualRadarBatchJobs(ctx, store.database, userID, batchID)
}

type manualRadarBatchQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func queryManualRadarBatchJobs(
	ctx context.Context,
	queryer manualRadarBatchQueryer,
	userID string,
	batchID string,
) ([]ManualRadarBatchJob, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT jq.id,jq.sequence,
		COALESCE(jq.user_id,''),COALESCE(jq.assignment_id,''),jq.job_type,jq.state,
		jq.due_at,jq.attempt_count,jq.max_attempts,jq.idempotency_key,jq.payload_json,
		COALESCE(rc.status,''),COALESCE((
			SELECT rd.state FROM radar_deliveries rd
			WHERE rd.candidate_id=rc.id AND rd.user_id=rc.user_id
			ORDER BY rd.created_at DESC LIMIT 1
		),'')
		FROM job_queue jq
		LEFT JOIN radar_candidates rc
			ON rc.id=json_extract(jq.payload_json,'$.candidate_id')
			AND rc.user_id=jq.user_id
		WHERE jq.user_id=? AND jq.job_type='deliver_radar'
			AND json_extract(jq.payload_json,'$.manual_batch_id')=?
		ORDER BY jq.sequence`, userID, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ManualRadarBatchJob
	for rows.Next() {
		var batchJob ManualRadarBatchJob
		var dueAt string
		if err := rows.Scan(
			&batchJob.Job.ID,
			&batchJob.Job.Sequence,
			&batchJob.Job.UserID,
			&batchJob.Job.AssignmentID,
			&batchJob.Job.Type,
			&batchJob.Job.State,
			&dueAt,
			&batchJob.Job.AttemptCount,
			&batchJob.Job.MaxAttempts,
			&batchJob.Job.IdempotencyKey,
			&batchJob.Job.PayloadJSON,
			&batchJob.CandidateState,
			&batchJob.DeliveryState,
		); err != nil {
			return nil, err
		}
		batchJob.Job.DueAt, err = parseTimestamp(dueAt)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, batchJob)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, sql.ErrNoRows
	}
	return jobs, nil
}
