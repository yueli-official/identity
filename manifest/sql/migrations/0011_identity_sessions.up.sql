CREATE TABLE identity_sessions (
    id          UUID PRIMARY KEY,
    identity_id UUID        NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen   TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_agent  TEXT        NOT NULL DEFAULT '',
    ip          TEXT        NOT NULL DEFAULT '',
    device      TEXT        NOT NULL DEFAULT '',
    expires_at  TIMESTAMPTZ
);

CREATE INDEX idx_identity_sessions_identity_id
    ON identity_sessions(identity_id, created_at DESC);

CREATE INDEX idx_identity_sessions_expires_at
    ON identity_sessions(expires_at);
