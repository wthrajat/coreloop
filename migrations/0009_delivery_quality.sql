UPDATE job_queue AS duplicate
SET state = 'cancelled',
    lease_owner = NULL,
    lease_expires_at = NULL,
    last_error_code = 'superseded_duplicate_source_poll',
    last_error_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE duplicate.job_type = 'ingest_source'
  AND duplicate.state IN ('queued', 'leased')
  AND EXISTS (
      SELECT 1
      FROM job_queue AS active
      WHERE active.job_type = 'ingest_source'
        AND active.state IN ('queued', 'leased')
        AND json_extract(active.payload_json, '$.source_id') =
            json_extract(duplicate.payload_json, '$.source_id')
        AND (
            (active.state = 'leased' AND duplicate.state = 'queued')
            OR (
                active.state = duplicate.state
                AND active.sequence < duplicate.sequence
            )
        )
  );

CREATE UNIQUE INDEX job_queue_one_active_source_poll_idx
    ON job_queue(json_extract(payload_json, '$.source_id'))
    WHERE job_type = 'ingest_source' AND state IN ('queued', 'leased');

INSERT OR IGNORE INTO prompt_versions
    (id, prompt_version, schema_version, compiler_version, instruction_checksum,
     schema_checksum, evaluation_status, approved_at)
VALUES
    ('prompt_lesson_v4', 'lesson-v4', 'lesson-draft-v1', 'compiler-v4',
     'runtime-verified', 'runtime-verified', 'approved',
     strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

INSERT INTO schema_migrations(version, name)
VALUES (9, 'delivery_quality');
