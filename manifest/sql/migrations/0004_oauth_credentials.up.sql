-- 0004_oauth_credentials.up.sql
-- Links an external OAuth identity (e.g. Google) to a local identity.
-- (provider, provider_uid) is the natural key used to recognise a returning user.
CREATE TABLE credentials_oauth (
    provider       TEXT        NOT NULL,
    provider_uid   TEXT        NOT NULL,
    identity_id    UUID        NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    email          TEXT,
    email_verified BOOLEAN     NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, provider_uid)
);
CREATE INDEX idx_credentials_oauth_identity ON credentials_oauth (identity_id);
