CREATE TABLE external_login_providers (
    key                  TEXT PRIMARY KEY CHECK (key IN ('google', 'qq')),
    client_id            TEXT NOT NULL,
    client_secret_cipher TEXT NOT NULL,
    secret_version       INTEGER NOT NULL CHECK (secret_version > 0),
    enabled              BOOLEAN NOT NULL DEFAULT FALSE,
    last_health_ok       BOOLEAN,
    last_health_checked_at TIMESTAMPTZ,
    last_health_error    TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE external_login_provider_events (
    id           BIGSERIAL PRIMARY KEY,
    provider_key TEXT NOT NULL REFERENCES external_login_providers(key),
    actor_id     TEXT NOT NULL,
    event        TEXT NOT NULL CHECK (event IN ('configured', 'health_checked')),
    metadata     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_external_login_provider_events_provider
    ON external_login_provider_events (provider_key, id DESC);
