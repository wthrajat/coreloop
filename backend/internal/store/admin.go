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

type SourceHealth struct {
	ID                  string `json:"id"`
	Publisher           string `json:"publisher"`
	FetchMethod         string `json:"fetch_method"`
	Role                string `json:"source_role"`
	PollState           string `json:"poll_state"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	LastPolledAt        string `json:"last_polled_at"`
	LastSuccessAt       string `json:"last_success_at"`
	LastErrorCode       string `json:"last_error_code"`
	LastErrorSummary    string `json:"last_error_summary"`
	LastErrorAt         string `json:"last_error_at"`
	LastItemCount       int    `json:"last_item_count"`
	RecentItems         int    `json:"recent_items"`
}

type Operations struct {
	Queued       int            `json:"queued"`
	Leased       int            `json:"leased"`
	Failed       int            `json:"failed"`
	BlockedQuota int            `json:"blocked_quota"`
	Users        int            `json:"users"`
	Sources      int            `json:"sources"`
	BlockedJobs  []BlockedJob   `json:"blocked_jobs"`
	FailedJobs   []FailedJob    `json:"failed_jobs"`
	SourceHealth []SourceHealth `json:"source_health"`
}

func (store *Store) Operations(ctx context.Context) (Operations, error) {
	value := Operations{
		BlockedJobs:  make([]BlockedJob, 0),
		FailedJobs:   make([]FailedJob, 0),
		SourceHealth: make([]SourceHealth, 0),
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
	if err := store.loadSourceHealth(ctx, &value); err != nil {
		return value, err
	}
	return value, nil
}

func (store *Store) loadSourceHealth(ctx context.Context, operations *Operations) error {
	rows, err := store.database.QueryContext(ctx, `WITH recent AS (
		SELECT source_id,COUNT(*) AS item_count
		FROM source_items
		WHERE retrieved_at>=strftime('%Y-%m-%dT%H:%M:%fZ','now','-10 days')
		GROUP BY source_id
	)
	SELECT s.id,s.publisher,s.fetch_method,s.source_role,s.last_poll_state,
		s.consecutive_failures,COALESCE(s.last_polled_at,''),COALESCE(s.last_success_at,''),
		COALESCE(s.last_error_code,''),COALESCE(s.last_error_summary,''),
		COALESCE(s.last_error_at,''),s.last_item_count,COALESCE(recent.item_count,0)
	FROM sources s LEFT JOIN recent ON recent.source_id=s.id
	WHERE s.enabled=1
	ORDER BY CASE s.last_poll_state
		WHEN 'failed' THEN 0 WHEN 'degraded' THEN 1 WHEN 'never' THEN 2 ELSE 3 END,
		s.consecutive_failures DESC,s.publisher`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var source SourceHealth
		if err := rows.Scan(
			&source.ID,
			&source.Publisher,
			&source.FetchMethod,
			&source.Role,
			&source.PollState,
			&source.ConsecutiveFailures,
			&source.LastPolledAt,
			&source.LastSuccessAt,
			&source.LastErrorCode,
			&source.LastErrorSummary,
			&source.LastErrorAt,
			&source.LastItemCount,
			&source.RecentItems,
		); err != nil {
			return err
		}
		operations.SourceHealth = append(operations.SourceHealth, source)
	}
	return rows.Err()
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
