ALTER TABLE job_queue
ADD COLUMN last_error_summary TEXT
    CHECK (
        last_error_summary IS NULL
        OR length(last_error_summary) BETWEEN 1 AND 800
    );

CREATE TABLE job_failure_events (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL REFERENCES job_queue(id) ON DELETE CASCADE,
    attempt_count INTEGER NOT NULL CHECK (attempt_count > 0),
    error_code TEXT NOT NULL CHECK (length(error_code) BETWEEN 1 AND 64),
    error_summary TEXT NOT NULL CHECK (length(error_summary) BETWEEN 1 AND 800),
    next_state TEXT NOT NULL
        CHECK (next_state IN ('queued', 'failed', 'blocked_quota')),
    occurred_at TEXT NOT NULL
);

CREATE INDEX job_failure_events_job_time_idx
    ON job_failure_events(job_id, occurred_at DESC);

CREATE TRIGGER job_queue_record_failure
AFTER UPDATE OF last_error_at ON job_queue
WHEN NEW.last_error_at IS NOT NULL
    AND NEW.last_error_summary IS NOT NULL
    AND (
        OLD.last_error_at IS NULL
        OR NEW.last_error_at <> OLD.last_error_at
    )
BEGIN
    INSERT INTO job_failure_events (
        id,
        job_id,
        attempt_count,
        error_code,
        error_summary,
        next_state,
        occurred_at
    ) VALUES (
        'failure_' || lower(hex(randomblob(16))),
        NEW.id,
        OLD.attempt_count,
        NEW.last_error_code,
        NEW.last_error_summary,
        NEW.state,
        NEW.last_error_at
    );
END;

INSERT INTO schema_migrations(version, name)
VALUES (10, 'job_failure_diagnostics');
