CREATE TABLE github_binding_attempts (
    id                   UUID PRIMARY KEY,
    state_digest         CHAR(64) NOT NULL UNIQUE,
    identity_id          UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    session_digest       CHAR(64) NOT NULL,
    verifier_ciphertext  TEXT NOT NULL,
    return_to            TEXT NOT NULL,
    expires_at           TIMESTAMPTZ NOT NULL,
    consumed_at          TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_github_binding_attempts_expiry
    ON github_binding_attempts (expires_at)
    WHERE consumed_at IS NULL;

CREATE TABLE github_identity_bindings (
    id                   UUID PRIMARY KEY,
    -- No FK by design: publisher subject history outlives account erasure.
    -- Identity's privacy finalizer scrubs all GitHub identifiers first.
    identity_id          UUID NOT NULL,
    provider             TEXT NOT NULL DEFAULT 'github' CHECK (provider = 'github'),
    provider_account_id  TEXT NOT NULL,
    provider_node_id     TEXT NOT NULL DEFAULT '',
    login_snapshot       TEXT NOT NULL,
    avatar_url_snapshot  TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL CHECK (status IN ('active', 'unbound', 'blocked')),
    verified_at          TIMESTAMPTZ NOT NULL,
    last_verified_at     TIMESTAMPTZ NOT NULL,
    unbound_at           TIMESTAMPTZ,
    blocked_at           TIMESTAMPTZ,
    erased_at            TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL,
    updated_at           TIMESTAMPTZ NOT NULL
);

-- A stable GitHub account can authorize at most one Identity at a time. Closed
-- rows remain as immutable history and do not prevent a fresh verified rebind.
CREATE UNIQUE INDEX uq_github_identity_bindings_active_account
    ON github_identity_bindings (provider, provider_account_id)
    WHERE status = 'active';

CREATE INDEX idx_github_identity_bindings_identity_history
    ON github_identity_bindings (identity_id, created_at DESC);
