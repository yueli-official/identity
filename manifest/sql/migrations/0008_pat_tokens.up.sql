-- 0008_pat_tokens.up.sql
-- Personal Access Tokens: opaque programmatic credentials bound to an identity.
-- Unlike audit_logs, PATs SHOULD die with their owner -> FK ON DELETE CASCADE.
-- token_hash is HMAC-SHA256(hex); plaintext is shown once at creation, never stored.
-- token_prefix is a non-secret display fragment (first 12 chars). scopes are opaque
-- strings interpreted by resource servers. Retention/cleanup of expired rows is a
-- later cron (idx_pat_tokens_expires supports a cheap time-based delete then).
CREATE TABLE pat_tokens (
    id           BIGSERIAL   PRIMARY KEY,
    identity_id  UUID        NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    token_hash   TEXT        NOT NULL UNIQUE,
    token_prefix TEXT        NOT NULL,
    scopes       JSONB       NOT NULL DEFAULT '[]',
    expires_at   TIMESTAMPTZ NULL,
    last_used_at TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_pat_tokens_identity ON pat_tokens (identity_id, id DESC);
CREATE INDEX idx_pat_tokens_expires  ON pat_tokens (expires_at);
