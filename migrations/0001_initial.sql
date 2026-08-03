PRAGMA foreign_keys = ON;

BEGIN IMMEDIATE;

CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    telegram_subject TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'paused', 'deleted')),
    locale TEXT NOT NULL DEFAULT 'en' CHECK (locale = 'en'),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at TEXT
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    rotated_at TEXT,
    revoked_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX sessions_user_active_idx
    ON sessions(user_id, expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE invites (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    expires_at TEXT NOT NULL,
    consumed_at TEXT,
    consumed_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (
        (consumed_at IS NULL AND consumed_by_user_id IS NULL)
        OR (consumed_at IS NOT NULL AND consumed_by_user_id IS NOT NULL)
    )
);

CREATE TABLE learning_profiles (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    current_level TEXT NOT NULL DEFAULT 'intermediate'
        CHECK (current_level IN ('beginner', 'intermediate', 'advanced')),
    goals_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(goals_json)),
    target_roles_json TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(target_roles_json)),
    current_technologies_json TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(current_technologies_json)),
    target_technologies_json TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(target_technologies_json)),
    explanation_style TEXT NOT NULL DEFAULT 'simple_technical'
        CHECK (explanation_style = 'simple_technical'),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE learning_preferences (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    lesson_minutes INTEGER NOT NULL DEFAULT 15
        CHECK (lesson_minutes IN (15, 30)),
    explanation_depth TEXT NOT NULL DEFAULT 'detailed'
        CHECK (explanation_depth IN ('foundation', 'standard', 'detailed')),
    lessons_per_day INTEGER NOT NULL DEFAULT 3
        CHECK (lessons_per_day BETWEEN 1 AND 6),
    radar_enabled INTEGER NOT NULL DEFAULT 1
        CHECK (radar_enabled IN (0, 1)),
    recall_mode TEXT NOT NULL DEFAULT 'light'
        CHECK (recall_mode IN ('off', 'light', 'standard')),
    weekends_enabled INTEGER NOT NULL DEFAULT 0
        CHECK (weekends_enabled IN (0, 1)),
    bundle_mode TEXT NOT NULL DEFAULT 'complete'
        CHECK (bundle_mode IN ('complete', 'continue_after_intro')),
    time_zone TEXT NOT NULL DEFAULT 'Asia/Kolkata',
    quiet_hours_start TEXT,
    quiet_hours_end TEXT,
    paused_until TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE delivery_schedules (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    day_of_week INTEGER NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    local_time TEXT NOT NULL CHECK (local_time GLOB '[0-2][0-9]:[0-5][0-9]'),
    time_zone TEXT NOT NULL DEFAULT 'Asia/Kolkata',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    active_from TEXT,
    active_until TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (user_id, day_of_week, local_time)
);

CREATE INDEX delivery_schedules_due_idx
    ON delivery_schedules(enabled, day_of_week, local_time);

CREATE TABLE delivery_destinations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel TEXT NOT NULL DEFAULT 'telegram' CHECK (channel = 'telegram'),
    telegram_chat_id TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'connected'
        CHECK (status IN ('connected', 'paused', 'disconnected')),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    connected_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (user_id, channel)
);

CREATE TABLE topics (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    lane TEXT NOT NULL,
    difficulty TEXT NOT NULL
        CHECK (difficulty IN ('beginner', 'intermediate', 'advanced')),
    prerequisites_json TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(prerequisites_json)),
    objectives_json TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(objectives_json)),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('draft', 'active', 'retired')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX topics_lane_status_idx ON topics(lane, status);

CREATE TABLE user_topic_preferences (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id TEXT NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL DEFAULT 50 CHECK (priority BETWEEN 0 AND 100),
    familiarity TEXT NOT NULL DEFAULT 'unknown'
        CHECK (familiarity IN ('unknown', 'weak', 'familiar', 'strong')),
    excluded INTEGER NOT NULL DEFAULT 0 CHECK (excluded IN (0, 1)),
    feedback_weight REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, topic_id)
);

CREATE TABLE learning_paths (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'paused', 'superseded')),
    rationale TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (user_id, version)
);

CREATE UNIQUE INDEX learning_paths_one_active_idx
    ON learning_paths(user_id)
    WHERE status = 'active';

CREATE TABLE theme_blocks (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    learning_path_id TEXT NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
    topic_id TEXT NOT NULL REFERENCES topics(id) ON DELETE RESTRICT,
    sequence_number INTEGER NOT NULL CHECK (sequence_number > 0),
    title TEXT NOT NULL,
    objectives_json TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(objectives_json)),
    planned_lesson_count INTEGER CHECK (planned_lesson_count > 0),
    status TEXT NOT NULL DEFAULT 'planned'
        CHECK (status IN ('planned', 'active', 'completed', 'paused', 'superseded')),
    selection_reason TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (learning_path_id, sequence_number)
);

CREATE UNIQUE INDEX theme_blocks_one_active_idx
    ON theme_blocks(user_id)
    WHERE status = 'active';

CREATE TABLE sources (
    id TEXT PRIMARY KEY,
    publisher TEXT NOT NULL,
    canonical_url TEXT NOT NULL UNIQUE,
    source_tier INTEGER NOT NULL CHECK (source_tier BETWEEN 1 AND 3),
    fetch_method TEXT NOT NULL
        CHECK (fetch_method IN ('rss', 'atom', 'api', 'html', 'manual')),
    trust_notes TEXT NOT NULL DEFAULT '',
    polling_interval_minutes INTEGER CHECK (polling_interval_minutes > 0),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    last_polled_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE source_items (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    canonical_url TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    published_at TEXT,
    updated_at_source TEXT,
    retrieved_at TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    evidence_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(evidence_json)),
    cluster_key TEXT,
    etag TEXT,
    last_modified TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX source_items_freshness_idx
    ON source_items(source_id, published_at DESC, retrieved_at DESC);
CREATE INDEX source_items_content_hash_idx ON source_items(content_hash);
CREATE INDEX source_items_cluster_idx ON source_items(cluster_key);

CREATE TABLE radar_candidates (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_item_id TEXT NOT NULL REFERENCES source_items(id) ON DELETE CASCADE,
    topic_id TEXT REFERENCES topics(id) ON DELETE SET NULL,
    ranker_version TEXT NOT NULL,
    relevance_score REAL NOT NULL,
    score_breakdown_json TEXT NOT NULL CHECK (json_valid(score_breakdown_json)),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'qualified', 'rejected', 'delivered', 'skipped')),
    rejection_reason TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (user_id, source_item_id, ranker_version)
);

CREATE INDEX radar_candidates_queue_idx
    ON radar_candidates(user_id, status, relevance_score DESC, created_at);

CREATE TABLE prompt_versions (
    id TEXT PRIMARY KEY,
    prompt_version TEXT NOT NULL UNIQUE,
    schema_version TEXT NOT NULL,
    compiler_version TEXT NOT NULL,
    instruction_checksum TEXT NOT NULL,
    schema_checksum TEXT NOT NULL,
    evaluation_status TEXT NOT NULL DEFAULT 'draft'
        CHECK (evaluation_status IN ('draft', 'approved', 'rejected', 'retired')),
    approved_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE lessons (
    id TEXT PRIMARY KEY,
    topic_id TEXT NOT NULL REFERENCES topics(id) ON DELETE RESTRICT,
    prompt_version_id TEXT NOT NULL REFERENCES prompt_versions(id) ON DELETE RESTRICT,
    lesson_type TEXT NOT NULL
        CHECK (lesson_type IN ('foundation', 'current_signal', 'product_decision', 'production_scenario', 'recall')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    title TEXT NOT NULL,
    estimated_reading_minutes INTEGER NOT NULL CHECK (estimated_reading_minutes > 0),
    normalized_content_json TEXT NOT NULL CHECK (json_valid(normalized_content_json)),
    content_fingerprint TEXT NOT NULL,
    cache_key TEXT NOT NULL,
    verification_state TEXT NOT NULL
        CHECK (verification_state IN ('verified', 'partially_verified', 'unverified_warning')),
    generation_state TEXT NOT NULL DEFAULT 'generated'
        CHECK (generation_state IN ('generated', 'validated', 'published', 'superseded')),
    generated_at TEXT NOT NULL,
    published_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (cache_key, version),
    UNIQUE (content_fingerprint, version)
);

CREATE INDEX lessons_topic_version_idx ON lessons(topic_id, version DESC);

CREATE TABLE lesson_source_items (
    lesson_id TEXT NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    source_item_id TEXT NOT NULL REFERENCES source_items(id) ON DELETE RESTRICT,
    evidence_order INTEGER NOT NULL CHECK (evidence_order > 0),
    PRIMARY KEY (lesson_id, source_item_id),
    UNIQUE (lesson_id, evidence_order)
);

CREATE TABLE lesson_parts (
    id TEXT PRIMARY KEY,
    lesson_id TEXT NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    sequence_number INTEGER NOT NULL CHECK (sequence_number > 0),
    total_parts INTEGER NOT NULL CHECK (total_parts > 0),
    rendered_text TEXT NOT NULL,
    character_count INTEGER NOT NULL CHECK (character_count BETWEEN 1 AND 4096),
    formatting_mode TEXT NOT NULL DEFAULT 'HTML'
        CHECK (formatting_mode IN ('HTML', 'MarkdownV2', 'plain')),
    renderer_version TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (lesson_id, sequence_number),
    CHECK (sequence_number <= total_parts)
);

CREATE TABLE lesson_assignments (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lesson_id TEXT NOT NULL REFERENCES lessons(id) ON DELETE RESTRICT,
    theme_block_id TEXT REFERENCES theme_blocks(id) ON DELETE SET NULL,
    schedule_position INTEGER CHECK (schedule_position > 0),
    assignment_reason TEXT NOT NULL
        CHECK (assignment_reason IN ('new', 'continuation', 'recall', 'remediation', 'material_update', 'requested')),
    state TEXT NOT NULL DEFAULT 'queued'
        CHECK (state IN ('queued', 'delivered', 'read', 'skipped', 'superseded')),
    assigned_at TEXT NOT NULL,
    delivered_at TEXT,
    read_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (user_id, theme_block_id, schedule_position)
);

CREATE INDEX lesson_assignments_backlog_idx
    ON lesson_assignments(user_id, state, assigned_at);

CREATE TABLE job_queue (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    assignment_id TEXT REFERENCES lesson_assignments(id) ON DELETE SET NULL,
    job_type TEXT NOT NULL
        CHECK (job_type IN ('generate_lesson', 'deliver_lesson', 'ingest_source', 'rank_radar', 'deliver_radar', 'recover')),
    state TEXT NOT NULL DEFAULT 'queued'
        CHECK (state IN ('queued', 'leased', 'completed', 'failed', 'blocked_quota', 'cancelled')),
    due_at TEXT NOT NULL,
    lease_owner TEXT,
    lease_expires_at TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    idempotency_key TEXT NOT NULL UNIQUE,
    payload_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(payload_json)),
    last_error_code TEXT,
    last_error_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (
        (state = 'leased' AND lease_owner IS NOT NULL AND lease_expires_at IS NOT NULL)
        OR state <> 'leased'
    )
);

CREATE INDEX job_queue_due_idx
    ON job_queue(state, due_at, sequence);
CREATE INDEX job_queue_lease_recovery_idx
    ON job_queue(state, lease_expires_at)
    WHERE state = 'leased';

CREATE TABLE provider_runs (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    job_id TEXT REFERENCES job_queue(id) ON DELETE SET NULL,
    lesson_id TEXT REFERENCES lessons(id) ON DELETE SET NULL,
    provider TEXT NOT NULL CHECK (provider IN ('groq', 'gemini', 'openai')),
    model_id TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    provider_request_id TEXT,
    request_kind TEXT NOT NULL
        CHECK (request_kind IN ('scheduled', 'corrective', 'manual_owner')),
    result_state TEXT NOT NULL
        CHECK (result_state IN ('succeeded', 'invalid', 'quota_exhausted', 'transient_error', 'permanent_error')),
    latency_ms INTEGER CHECK (latency_ms >= 0),
    input_tokens INTEGER CHECK (input_tokens >= 0),
    output_tokens INTEGER CHECK (output_tokens >= 0),
    cache_read_tokens INTEGER CHECK (cache_read_tokens >= 0),
    cache_write_tokens INTEGER CHECK (cache_write_tokens >= 0),
    estimated_cost_microusd INTEGER CHECK (estimated_cost_microusd >= 0),
    validation_result_json TEXT NOT NULL DEFAULT '{}'
        CHECK (json_valid(validation_result_json)),
    error_code TEXT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (provider, provider_request_id)
);

CREATE INDEX provider_runs_job_idx ON provider_runs(job_id, started_at);

CREATE TABLE deliveries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assignment_id TEXT NOT NULL REFERENCES lesson_assignments(id) ON DELETE CASCADE,
    destination_id TEXT NOT NULL REFERENCES delivery_destinations(id) ON DELETE RESTRICT,
    job_id TEXT REFERENCES job_queue(id) ON DELETE SET NULL,
    intended_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    bundle_state TEXT NOT NULL DEFAULT 'pending'
        CHECK (bundle_state IN ('pending', 'sending', 'delivered', 'partial', 'failed', 'blocked_quota')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    idempotency_key TEXT NOT NULL UNIQUE,
    last_error_code TEXT,
    last_error_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX deliveries_user_state_idx
    ON deliveries(user_id, bundle_state, intended_at);

CREATE TABLE delivery_parts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    delivery_id TEXT NOT NULL REFERENCES deliveries(id) ON DELETE CASCADE,
    lesson_part_id TEXT NOT NULL REFERENCES lesson_parts(id) ON DELETE RESTRICT,
    sequence_number INTEGER NOT NULL CHECK (sequence_number > 0),
    state TEXT NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'sending', 'delivered', 'failed')),
    idempotency_key TEXT NOT NULL UNIQUE,
    telegram_message_id TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    last_error_code TEXT,
    sent_at TEXT,
    confirmed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (delivery_id, sequence_number)
);

CREATE TABLE interactions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assignment_id TEXT REFERENCES lesson_assignments(id) ON DELETE CASCADE,
    radar_candidate_id TEXT REFERENCES radar_candidates(id) ON DELETE CASCADE,
    action TEXT NOT NULL
        CHECK (action IN ('read', 'save', 'skip', 'not_relevant', 'already_know', 'next', 'deeper', 'example', 'quiz', 'sources', 'pause', 'settings', 'continue')),
    idempotency_key TEXT NOT NULL UNIQUE,
    answer_text TEXT,
    answer_correct INTEGER CHECK (answer_correct IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (assignment_id IS NOT NULL OR radar_candidate_id IS NOT NULL)
);

CREATE INDEX interactions_user_action_idx
    ON interactions(user_id, action, created_at DESC);

CREATE TABLE reviews (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assignment_id TEXT NOT NULL REFERENCES lesson_assignments(id) ON DELETE CASCADE,
    question_text TEXT NOT NULL,
    expected_answer_json TEXT NOT NULL CHECK (json_valid(expected_answer_json)),
    due_at TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'due'
        CHECK (state IN ('scheduled', 'due', 'answered', 'skipped')),
    result TEXT CHECK (result IN ('correct', 'partial', 'incorrect')),
    answered_at TEXT,
    next_review_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX reviews_due_idx ON reviews(user_id, state, due_at);

CREATE TABLE decisions (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    scope TEXT NOT NULL CHECK (scope IN ('product', 'architecture', 'operations')),
    title TEXT NOT NULL,
    decision_text TEXT NOT NULL,
    evidence_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(evidence_json)),
    status TEXT NOT NULL DEFAULT 'accepted'
        CHECK (status IN ('proposed', 'accepted', 'superseded')),
    decided_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE cache_entries (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    cache_type TEXT NOT NULL
        CHECK (cache_type IN ('source_http', 'normalized_source', 'radar', 'lesson', 'prompt_input', 'render', 'interactive')),
    cache_key TEXT NOT NULL,
    value_reference TEXT NOT NULL,
    source_version TEXT,
    prompt_version TEXT,
    verification_state TEXT NOT NULL
        CHECK (verification_state IN ('verified', 'not_applicable')),
    expires_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (cache_type, cache_key)
);

CREATE INDEX cache_entries_expiry_idx
    ON cache_entries(cache_type, expires_at)
    WHERE expires_at IS NOT NULL;

INSERT INTO schema_migrations (version, name)
VALUES (1, 'initial');

COMMIT;
