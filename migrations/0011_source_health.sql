ALTER TABLE sources ADD COLUMN last_poll_state TEXT NOT NULL DEFAULT 'never'
    CHECK (last_poll_state IN ('never', 'healthy', 'degraded', 'failed'));
ALTER TABLE sources ADD COLUMN last_success_at TEXT;
ALTER TABLE sources ADD COLUMN last_error_code TEXT
    CHECK (last_error_code IS NULL OR length(last_error_code) BETWEEN 1 AND 64);
ALTER TABLE sources ADD COLUMN last_error_summary TEXT
    CHECK (last_error_summary IS NULL OR length(last_error_summary) BETWEEN 1 AND 500);
ALTER TABLE sources ADD COLUMN last_error_at TEXT;
ALTER TABLE sources ADD COLUMN last_item_count INTEGER NOT NULL DEFAULT 0
    CHECK (last_item_count >= 0);

UPDATE sources
SET last_poll_state = CASE
        WHEN consecutive_failures > 0 THEN 'failed'
        WHEN last_polled_at IS NOT NULL THEN 'healthy'
        ELSE 'never'
    END,
    last_success_at = CASE
        WHEN consecutive_failures = 0 THEN last_polled_at
        ELSE NULL
    END,
    last_error_code = CASE
        WHEN consecutive_failures > 0 THEN 'historical_poll_failure'
        ELSE NULL
    END,
    last_error_summary = CASE
        WHEN consecutive_failures > 0
            THEN 'This source failed before detailed diagnostics were available.'
        ELSE NULL
    END,
    last_error_at = CASE
        WHEN consecutive_failures > 0 THEN last_polled_at
        ELSE NULL
    END;

CREATE INDEX sources_health_idx
    ON sources(enabled, last_poll_state, consecutive_failures DESC, publisher);

-- Re-evaluate a bounded, source-balanced slice of the current ten-day window
-- under deterministic-editorial-v4. The jobs keep normal chronological queue
-- order and are idempotent if this migration is retried.
WITH source_fresh AS (
    SELECT
        si.id,
        ROW_NUMBER() OVER (
            PARTITION BY si.source_id
            ORDER BY COALESCE(si.published_at, si.retrieved_at) DESC, si.id
        ) AS source_position
    FROM source_items si
    WHERE COALESCE(si.published_at, si.retrieved_at) >=
        strftime('%Y-%m-%dT%H:%M:%fZ', 'now', '-10 days')
), bounded AS (
    SELECT
        id,
        (ROW_NUMBER() OVER (ORDER BY id) - 1) / 25 AS batch_number
    FROM source_fresh
    WHERE source_position <= 25
), batches AS (
    SELECT
        batch_number,
        json_object(
            'source_item_ids',
            json(json_group_array(id))
        ) AS payload_json
    FROM bounded
    GROUP BY batch_number
)
INSERT OR IGNORE INTO job_queue (
    id,
    job_type,
    due_at,
    idempotency_key,
    payload_json
)
SELECT
    'job_radar_reindex_v4_' || printf('%03d', batch_number),
    'rank_radar',
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    'radar-reindex:deterministic-editorial-v4:' || batch_number,
    payload_json
FROM batches;

INSERT INTO schema_migrations(version, name)
VALUES (11, 'source_health');
