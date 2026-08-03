CREATE TABLE auth_flows (
    id TEXT PRIMARY KEY,
    state_hash TEXT NOT NULL UNIQUE,
    invite_id TEXT REFERENCES invites(id) ON DELETE CASCADE,
    code_verifier TEXT NOT NULL,
    nonce TEXT NOT NULL,
    return_path TEXT NOT NULL DEFAULT '/onboarding',
    expires_at TEXT NOT NULL,
    used_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX auth_flows_expiry_idx ON auth_flows(expires_at) WHERE used_at IS NULL;

ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN username TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN csrf_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE sources ADD COLUMN etag TEXT;
ALTER TABLE sources ADD COLUMN last_modified TEXT;
ALTER TABLE sources ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0;

CREATE TABLE notification_events (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('welcome', 'quota_exhausted', 'admin_alert', 'account_deleted')),
    deduplication_key TEXT NOT NULL UNIQUE,
    delivered_at TEXT,
    last_error_code TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE rate_limits (
    bucket_key TEXT PRIMARY KEY,
    window_started_at TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0 CHECK (request_count >= 0),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX lessons_cache_idx ON lessons(cache_key, generation_state, created_at DESC);
CREATE INDEX provider_runs_provider_state_idx ON provider_runs(provider, result_state, completed_at DESC);
CREATE INDEX notification_events_user_kind_idx ON notification_events(user_id, kind, created_at DESC);
