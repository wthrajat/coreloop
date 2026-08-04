package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"coreloop/backend/internal/ids"
)

// ErrJobNotLeasable means a duplicate wake found an existing job that another
// worker owns or that the durable queue has already rescheduled or finished.
var ErrJobNotLeasable = errors.New("job is not currently leasable")

func (store *Store) EnqueueJob(ctx context.Context, userID, assignmentID, jobType string, dueAt time.Time, idempotencyKey string, payload any) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	id, err := ids.New("job")
	if err != nil {
		return "", err
	}
	var user, assignment any
	if userID != "" {
		user = userID
	}
	if assignmentID != "" {
		assignment = assignmentID
	}
	_, err = store.database.ExecContext(ctx, `INSERT INTO job_queue
		(id,user_id,assignment_id,job_type,due_at,idempotency_key,payload_json)
		VALUES (?,?,?,?,?,?,?) ON CONFLICT(idempotency_key) DO NOTHING`, id, user, assignment, jobType, timestamp(dueAt), idempotencyKey, string(encoded))
	if err != nil {
		return "", err
	}
	err = store.database.QueryRowContext(ctx, "SELECT id FROM job_queue WHERE idempotency_key=?", idempotencyKey).Scan(&id)
	return id, err
}

type DueOccurrence struct {
	UserID string
	At     time.Time
	Key    string
}

func (store *Store) DueOccurrences(ctx context.Context, now time.Time, tolerance time.Duration) ([]DueOccurrence, error) {
	rows, err := store.database.QueryContext(ctx, `SELECT ds.user_id,ds.day_of_week,ds.local_time,ds.time_zone
		FROM delivery_schedules ds JOIN users u ON u.id=ds.user_id
		JOIN learning_preferences lp ON lp.user_id=ds.user_id
		JOIN delivery_destinations dd ON dd.user_id=ds.user_id AND dd.channel='telegram'
			AND dd.enabled=1 AND dd.status='connected'
		WHERE ds.enabled=1 AND u.status='active' AND (lp.paused_until IS NULL OR lp.paused_until<=?)`, timestamp(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var output []DueOccurrence
	for rows.Next() {
		var userID, localTime, zone string
		var day int
		if err := rows.Scan(&userID, &day, &localTime, &zone); err != nil {
			return nil, err
		}
		location, err := time.LoadLocation(zone)
		if err != nil {
			return nil, err
		}
		localNow := now.In(location)
		if int(localNow.Weekday()) != day {
			continue
		}
		var hour, minute int
		if _, err := fmt.Sscanf(localTime, "%d:%d", &hour, &minute); err != nil {
			return nil, err
		}
		occurrence := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, minute, 0, 0, location)
		delta := now.Sub(occurrence.UTC())
		if delta < 0 {
			delta = -delta
		}
		if delta <= tolerance {
			output = append(output, DueOccurrence{UserID: userID, At: occurrence.UTC(), Key: "lesson:" + userID + ":" + occurrence.Format("20060102T1504")})
		}
	}
	return output, rows.Err()
}

func (store *Store) EnqueueSourcePolls(ctx context.Context, now time.Time) error {
	rows, err := store.database.QueryContext(ctx, `SELECT id,polling_interval_minutes,last_polled_at FROM sources WHERE enabled=1`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type source struct {
		id       string
		interval int
		last     sql.NullString
	}
	var sources []source
	for rows.Next() {
		var value source
		if err := rows.Scan(&value.id, &value.interval, &value.last); err != nil {
			return err
		}
		sources = append(sources, value)
	}
	for _, source := range sources {
		due := true
		if source.last.Valid {
			last, err := parseTimestamp(source.last.String)
			if err == nil && last.Add(time.Duration(source.interval)*time.Minute).After(now) {
				due = false
			}
		}
		if due {
			bucket := now.UTC().Truncate(time.Duration(source.interval) * time.Minute).Format(time.RFC3339)
			_, err := store.EnqueueJob(ctx, "", "", "ingest_source", now, "source:"+source.id+":"+bucket, map[string]string{"source_id": source.id})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (store *Store) RecoverJobs(ctx context.Context, now time.Time) error {
	_, err := store.database.ExecContext(ctx, `UPDATE job_queue SET state='queued',lease_owner=NULL,lease_expires_at=NULL,
		updated_at=? WHERE state='leased' AND lease_expires_at<=?`, timestamp(now), timestamp(now))
	if err != nil {
		return err
	}
	_, err = store.database.ExecContext(ctx, `UPDATE job_queue SET state='queued',due_at=?,updated_at=?
		WHERE state='blocked_quota' AND due_at<=?`, timestamp(now), timestamp(now), timestamp(now))
	if err != nil {
		return err
	}
	_, _ = store.database.ExecContext(ctx, `DELETE FROM auth_flows WHERE expires_at<?`, timestamp(now.Add(-24*time.Hour)))
	_, _ = store.database.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<? OR revoked_at<?`, timestamp(now.Add(-24*time.Hour)), timestamp(now.Add(-30*24*time.Hour)))
	return nil
}

func (store *Store) PublishableJobs(ctx context.Context, now time.Time, limit int) ([]Job, error) {
	if limit <= 0 || limit > 25 {
		limit = 10
	}
	rows, err := store.database.QueryContext(ctx, `SELECT id,sequence,COALESCE(user_id,''),COALESCE(assignment_id,''),job_type,state,due_at,attempt_count,max_attempts,idempotency_key,payload_json
		FROM job_queue WHERE state='queued' AND due_at<=? ORDER BY sequence LIMIT ?`, timestamp(now), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanJob(row scanner) (Job, error) {
	var job Job
	var due string
	err := row.Scan(&job.ID, &job.Sequence, &job.UserID, &job.AssignmentID, &job.Type, &job.State, &due, &job.AttemptCount, &job.MaxAttempts, &job.IdempotencyKey, &job.PayloadJSON)
	if err != nil {
		return Job{}, err
	}
	job.DueAt, err = parseTimestamp(due)
	return job, err
}

func (store *Store) LeaseJob(ctx context.Context, jobID, owner string, now time.Time) (Job, error) {
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE job_queue SET state='leased',lease_owner=?,lease_expires_at=?,attempt_count=attempt_count+1,updated_at=?
		WHERE id=? AND state='queued' AND due_at<=? AND attempt_count<max_attempts`, owner, timestamp(now.Add(4*time.Minute)), timestamp(now), jobID, timestamp(now))
	if err != nil {
		return Job{}, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		var state string
		err := tx.QueryRowContext(ctx, "SELECT state FROM job_queue WHERE id=?", jobID).Scan(&state)
		if err != nil {
			return Job{}, err
		}
		if err := tx.Commit(); err != nil {
			return Job{}, err
		}
		return Job{ID: jobID, State: state}, ErrJobNotLeasable
	}
	row := tx.QueryRowContext(ctx, `SELECT id,sequence,COALESCE(user_id,''),COALESCE(assignment_id,''),job_type,state,due_at,attempt_count,max_attempts,idempotency_key,payload_json FROM job_queue WHERE id=?`, jobID)
	job, err := scanJob(row)
	if err != nil {
		return Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (store *Store) CompleteJob(ctx context.Context, jobID string, now time.Time) error {
	_, err := store.database.ExecContext(ctx, `UPDATE job_queue SET state='completed',lease_owner=NULL,lease_expires_at=NULL,completed_at=?,updated_at=? WHERE id=?`, timestamp(now), timestamp(now), jobID)
	return err
}

func (store *Store) LinkJobAssignment(ctx context.Context, jobID, assignmentID string, now time.Time) error {
	_, err := store.database.ExecContext(ctx, `UPDATE job_queue SET assignment_id=COALESCE(assignment_id,?),updated_at=? WHERE id=?`, assignmentID, timestamp(now), jobID)
	return err
}

func (store *Store) AssignmentDeliveryState(ctx context.Context, userID, assignmentID string) (string, string, error) {
	var assignmentState, deliveryState string
	err := store.database.QueryRowContext(ctx, `SELECT la.state,COALESCE((
		SELECT jq.state FROM job_queue jq
		WHERE jq.assignment_id=la.id AND jq.job_type='deliver_lesson'
		ORDER BY jq.sequence DESC LIMIT 1
	),'') FROM lesson_assignments la WHERE la.id=? AND la.user_id=?`, assignmentID, userID).Scan(&assignmentState, &deliveryState)
	return assignmentState, deliveryState, err
}

func (store *Store) FailJob(ctx context.Context, job Job, code string, quota bool, now time.Time) error {
	state := "queued"
	due := now.Add(time.Duration(job.AttemptCount*job.AttemptCount) * time.Minute)
	if quota {
		state = "blocked_quota"
		due = now.Add(time.Hour)
	} else if job.AttemptCount >= job.MaxAttempts {
		state = "failed"
	}
	_, err := store.database.ExecContext(ctx, `UPDATE job_queue SET state=?,due_at=?,lease_owner=NULL,lease_expires_at=NULL,last_error_code=?,last_error_at=?,updated_at=? WHERE id=?`, state, timestamp(due), code, timestamp(now), timestamp(now), job.ID)
	return err
}

func (store *Store) Job(ctx context.Context, jobID string) (Job, error) {
	return scanJob(store.database.QueryRowContext(ctx, `SELECT id,sequence,COALESCE(user_id,''),COALESCE(assignment_id,''),job_type,state,due_at,attempt_count,max_attempts,idempotency_key,payload_json FROM job_queue WHERE id=?`, jobID))
}

func (store *Store) RecordProviderRun(ctx context.Context, job Job, generatedBy, model, requestID, requestKind, state, errorCode string, input, output int, started, completed time.Time) error {
	id, err := ids.New("run")
	if err != nil {
		return err
	}
	var user any
	if job.UserID != "" {
		user = job.UserID
	}
	var request any
	if requestID != "" {
		request = requestID
	}
	_, err = store.database.ExecContext(ctx, `INSERT INTO provider_runs
		(id,user_id,job_id,provider,model_id,prompt_version,schema_version,provider_request_id,request_kind,result_state,input_tokens,output_tokens,error_code,started_at,completed_at)
		VALUES (?,?,?,?,?,'lesson-v1','lesson-draft-v1',?,?,?,?,?,?,?,?)`, id, user, job.ID, generatedBy, model, request, requestKind, state, input, output, errorCode, timestamp(started), timestamp(completed))
	return err
}
