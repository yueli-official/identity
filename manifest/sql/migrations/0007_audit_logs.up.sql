-- 0007_audit_logs.up.sql
-- Append-only security audit trail for identity lifecycle events. No FK to
-- identities: audit must outlive identity deletion and stay immutable/independent.
-- Retention ~90d (rotation/purge job is a later milestone; idx_audit_logs_occurred
-- supports a cheap time-based delete then).
CREATE TABLE audit_logs (
    id                 BIGSERIAL   PRIMARY KEY,
    event              TEXT        NOT NULL,
    actor_identity_id  UUID        NULL,
    target_identity_id UUID        NULL,
    actor_email        TEXT        NULL,
    ip                 TEXT        NULL,
    user_agent         TEXT        NULL,
    client_id          TEXT        NULL,
    result             TEXT        NOT NULL DEFAULT 'success',
    detail             JSONB       NOT NULL DEFAULT '{}',
    occurred_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_target   ON audit_logs (target_identity_id, occurred_at DESC);
CREATE INDEX idx_audit_logs_actor    ON audit_logs (actor_identity_id, occurred_at DESC);
CREATE INDEX idx_audit_logs_event    ON audit_logs (event, occurred_at DESC);
CREATE INDEX idx_audit_logs_occurred ON audit_logs (occurred_at);
