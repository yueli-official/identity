-- Identity-service schema (milestone 2). UUID ids, no tenant_id (single-tenant).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE identities (
    id             UUID PRIMARY KEY,
    email          TEXT NOT NULL,            -- canonical (lowercased, trimmed)
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    status         TEXT NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'disabled', 'deleted')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Email is unique only among non-deleted identities (soft-delete frees the email).
CREATE UNIQUE INDEX uq_identities_email_active
    ON identities (email) WHERE status <> 'deleted';

CREATE TABLE user_profiles (
    identity_id  UUID PRIMARY KEY REFERENCES identities (id) ON DELETE CASCADE,
    username     TEXT,
    display_name TEXT,
    avatar_url   TEXT,
    locale       TEXT NOT NULL DEFAULT 'zh-CN',
    extensions   JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_user_profiles_username
    ON user_profiles (username) WHERE username IS NOT NULL;

CREATE TABLE credentials_password (
    identity_id   UUID PRIMARY KEY REFERENCES identities (id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,             -- bcrypt
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
