package store

import (
	"context"
)

type BlockedJob struct {
	ID           string `json:"id"`
	CreatedAt    string `json:"created_at"`
	AttemptCount int    `json:"attempt_count"`
}
type Operations struct {
	Queued       int          `json:"queued"`
	Leased       int          `json:"leased"`
	Failed       int          `json:"failed"`
	BlockedQuota int          `json:"blocked_quota"`
	Users        int          `json:"users"`
	Sources      int          `json:"sources"`
	BlockedJobs  []BlockedJob `json:"blocked_jobs"`
}

func (store *Store) Operations(ctx context.Context) (Operations, error) {
	var value Operations
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
	rows, err := store.database.QueryContext(ctx, `SELECT id,created_at,attempt_count FROM job_queue WHERE state='blocked_quota' ORDER BY sequence LIMIT 50`)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	for rows.Next() {
		var job BlockedJob
		if err := rows.Scan(&job.ID, &job.CreatedAt, &job.AttemptCount); err != nil {
			return value, err
		}
		value.BlockedJobs = append(value.BlockedJobs, job)
	}
	return value, rows.Err()
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
