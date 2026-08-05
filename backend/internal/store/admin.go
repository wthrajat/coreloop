package store

import (
	"context"
)

type BlockedJob struct {
	ID           string `json:"id"`
	CreatedAt    string `json:"created_at"`
	AttemptCount int    `json:"attempt_count"`
}

type JobFailureEvent struct {
	AttemptCount int    `json:"attempt_count"`
	ErrorCode    string `json:"error_code"`
	ErrorSummary string `json:"error_summary"`
	NextState    string `json:"next_state"`
	OccurredAt   string `json:"occurred_at"`
}

type FailedJob struct {
	ID               string            `json:"id"`
	Type             string            `json:"job_type"`
	CreatedAt        string            `json:"created_at"`
	FailedAt         string            `json:"failed_at"`
	AttemptCount     int               `json:"attempt_count"`
	MaxAttempts      int               `json:"max_attempts"`
	LastErrorCode    string            `json:"last_error_code"`
	LastErrorSummary string            `json:"last_error_summary"`
	FailureCount     int               `json:"failure_count"`
	Failures         []JobFailureEvent `json:"failures"`
}

type Operations struct {
	Queued       int          `json:"queued"`
	Leased       int          `json:"leased"`
	Failed       int          `json:"failed"`
	BlockedQuota int          `json:"blocked_quota"`
	Users        int          `json:"users"`
	Sources      int          `json:"sources"`
	BlockedJobs  []BlockedJob `json:"blocked_jobs"`
	FailedJobs   []FailedJob  `json:"failed_jobs"`
}

func (store *Store) Operations(ctx context.Context) (Operations, error) {
	value := Operations{
		BlockedJobs: make([]BlockedJob, 0),
		FailedJobs:  make([]FailedJob, 0),
	}
	if err := store.database.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM job_queue WHERE state='queued'),
		(SELECT COUNT(*) FROM job_queue WHERE state='leased'),
		(SELECT COUNT(*) FROM job_queue WHERE state='failed'),
		(SELECT COUNT(*) FROM job_queue WHERE state='blocked_quota'),
		(SELECT COUNT(*) FROM users WHERE status='active'),
		(SELECT COUNT(*) FROM sources WHERE enabled=1)`).Scan(
		&value.Queued, &value.Leased, &value.Failed, &value.BlockedQuota,
		&value.Users, &value.Sources,
	); err != nil {
		return value, err
	}
	rows, err := store.database.QueryContext(ctx, `SELECT id,created_at,attempt_count
		FROM job_queue WHERE state='blocked_quota' ORDER BY sequence LIMIT 50`)
	if err != nil {
		return value, err
	}
	for rows.Next() {
		var job BlockedJob
		if err := rows.Scan(&job.ID, &job.CreatedAt, &job.AttemptCount); err != nil {
			return value, err
		}
		value.BlockedJobs = append(value.BlockedJobs, job)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return value, err
	}
	if err := rows.Close(); err != nil {
		return value, err
	}
	if err := store.loadFailedJobs(ctx, &value); err != nil {
		return value, err
	}
	return value, nil
}

func (store *Store) loadFailedJobs(ctx context.Context, operations *Operations) error {
	rows, err := store.database.QueryContext(ctx, `SELECT
		id,job_type,created_at,COALESCE(last_error_at,updated_at),attempt_count,max_attempts,
		COALESCE(last_error_code,''),COALESCE(last_error_summary,''),
		(SELECT COUNT(*) FROM job_failure_events WHERE job_id=job_queue.id)
		FROM job_queue WHERE state='failed'
		ORDER BY COALESCE(last_error_at,updated_at) DESC,sequence DESC LIMIT 50`)
	if err != nil {
		return err
	}
	jobIndexes := make(map[string]int)
	for rows.Next() {
		var job FailedJob
		job.Failures = make([]JobFailureEvent, 0)
		if err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.CreatedAt,
			&job.FailedAt,
			&job.AttemptCount,
			&job.MaxAttempts,
			&job.LastErrorCode,
			&job.LastErrorSummary,
			&job.FailureCount,
		); err != nil {
			rows.Close()
			return err
		}
		jobIndexes[job.ID] = len(operations.FailedJobs)
		operations.FailedJobs = append(operations.FailedJobs, job)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(operations.FailedJobs) == 0 {
		return nil
	}

	eventRows, err := store.database.QueryContext(ctx, `SELECT
		job_id,attempt_count,error_code,error_summary,next_state,occurred_at
		FROM (
			SELECT event.*,
				ROW_NUMBER() OVER (
					PARTITION BY event.job_id
					ORDER BY event.occurred_at DESC,event.id DESC
				) AS failure_rank
			FROM job_failure_events event
			WHERE event.job_id IN (
				SELECT id FROM job_queue WHERE state='failed'
				ORDER BY COALESCE(last_error_at,updated_at) DESC,sequence DESC LIMIT 50
			)
		)
		WHERE failure_rank<=20
		ORDER BY occurred_at DESC`)
	if err != nil {
		return err
	}
	defer eventRows.Close()
	for eventRows.Next() {
		var jobID string
		var event JobFailureEvent
		if err := eventRows.Scan(
			&jobID,
			&event.AttemptCount,
			&event.ErrorCode,
			&event.ErrorSummary,
			&event.NextState,
			&event.OccurredAt,
		); err != nil {
			return err
		}
		if index, ok := jobIndexes[jobID]; ok {
			operations.FailedJobs[index].Failures = append(
				operations.FailedJobs[index].Failures,
				event,
			)
		}
	}
	return eventRows.Err()
}

func (store *Store) ExportUser(ctx context.Context, userID string) (map[string]any, error) {
	profile, preferences, err := store.Profile(ctx, userID)
	if err != nil {
		return nil, err
	}
	assignments, err := store.Assignments(ctx, userID, 1000)
	if err != nil {
		return nil, err
	}
	topics, err := store.Topics(ctx)
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, id := range preferences.TopicIDs {
		selected[id] = true
	}
	var chosen []Topic
	for _, topic := range topics {
		if selected[topic.ID] {
			chosen = append(chosen, topic)
		}
	}
	return map[string]any{"profile": profile, "preferences": preferences, "topics": chosen, "assignments": assignments}, nil
}
