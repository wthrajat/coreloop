CREATE INDEX job_queue_user_state_sequence_idx
    ON job_queue(user_id, state, sequence);

CREATE INDEX job_queue_assignment_type_sequence_idx
    ON job_queue(assignment_id, job_type, sequence DESC);

CREATE INDEX job_queue_user_type_sequence_idx
    ON job_queue(user_id, job_type, sequence DESC);

CREATE INDEX radar_candidates_cleanup_idx
    ON radar_candidates(status, ranker_version, relevance_score, source_item_id);

CREATE INDEX rate_limits_updated_at_idx ON rate_limits(updated_at);

ALTER TABLE lesson_assignments
    ADD COLUMN recall_review_id TEXT REFERENCES reviews(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX lesson_assignments_recall_review_idx
    ON lesson_assignments(recall_review_id)
    WHERE recall_review_id IS NOT NULL;
