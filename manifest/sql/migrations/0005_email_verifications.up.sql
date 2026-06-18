-- 0005_email_verifications.up.sql
-- One row per issued email token. purpose isolates scopes (verify_email vs password_reset,
-- spec §11 "登录码 ≠ 找回码"). Tokens are stored hashed; single-use via used_at; TTL via expires_at.
CREATE TABLE email_verifications (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_id UUID        NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    email       TEXT        NOT NULL,
    purpose     TEXT        NOT NULL,   -- 'verify_email' | 'password_reset'
    token_hash  TEXT        NOT NULL,   -- sha256 hex of the opaque token
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_email_verifications_token ON email_verifications (token_hash);
CREATE INDEX idx_email_verifications_identity ON email_verifications (identity_id);
