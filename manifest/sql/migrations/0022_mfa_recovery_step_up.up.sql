CREATE TABLE authentication_policies (
    identity_id             UUID PRIMARY KEY REFERENCES identities(id) ON DELETE CASCADE,
    second_factor_required  BOOLEAN NOT NULL DEFAULT FALSE,
    policy_version          INTEGER NOT NULL DEFAULT 1 CHECK (policy_version > 0),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE totp_authenticators (
    id                    UUID PRIMARY KEY,
    identity_id           UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    label                 TEXT NOT NULL DEFAULT '',
    secret_ciphertext     BYTEA NOT NULL CHECK (octet_length(secret_ciphertext) >= 29),
    key_version           INTEGER NOT NULL CHECK (key_version > 0),
    algorithm             TEXT NOT NULL DEFAULT 'SHA1'
                          CHECK (algorithm IN ('SHA1', 'SHA256', 'SHA512')),
    digits                SMALLINT NOT NULL DEFAULT 6 CHECK (digits IN (6, 8)),
    period_seconds        INTEGER NOT NULL DEFAULT 30 CHECK (period_seconds BETWEEN 15 AND 120),
    status                TEXT NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending', 'active', 'suspended', 'revoked')),
    binding_session_id    UUID,
    enrollment_expires_at TIMESTAMPTZ,
    failed_attempts       INTEGER NOT NULL DEFAULT 0 CHECK (failed_attempts BETWEEN 0 AND 5),
    last_used_step        BIGINT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    verified_at           TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at          TIMESTAMPTZ,
    revoked_at            TIMESTAMPTZ,
    revoked_reason        TEXT NOT NULL DEFAULT '',
    CHECK (
        (status = 'pending' AND binding_session_id IS NOT NULL AND enrollment_expires_at IS NOT NULL)
        OR
        (status <> 'pending' AND binding_session_id IS NULL AND enrollment_expires_at IS NULL)
    )
);

CREATE UNIQUE INDEX totp_authenticators_one_pending_per_identity_idx
    ON totp_authenticators(identity_id) WHERE status = 'pending';
CREATE INDEX totp_authenticators_identity_status_idx
    ON totp_authenticators(identity_id, status, created_at DESC);

CREATE TABLE recovery_code_sets (
    id          UUID PRIMARY KEY,
    identity_id UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active', 'exhausted', 'revoked')),
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    exhausted_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    revoked_reason TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX recovery_code_sets_one_active_per_identity_idx
    ON recovery_code_sets(identity_id) WHERE status = 'active';

CREATE TABLE recovery_codes (
    id          UUID PRIMARY KEY,
    set_id      UUID NOT NULL REFERENCES recovery_code_sets(id) ON DELETE CASCADE,
    code_digest BYTEA NOT NULL CHECK (octet_length(code_digest) = 32),
    consumed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (set_id, code_digest)
);

CREATE INDEX recovery_codes_available_idx
    ON recovery_codes(set_id) WHERE consumed_at IS NULL;

-- General authentication transactions back MFA, recovery and future
-- action-bound step-up. The browser receives only the opaque id; state and
-- requirements remain server-owned and single-use.
CREATE TABLE authentication_transactions (
    id               UUID PRIMARY KEY,
    kind             TEXT NOT NULL
                     CHECK (kind IN ('mfa_login', 'recovery', 'step_up')),
    identity_id      UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    session_id       UUID,
    audience         TEXT NOT NULL DEFAULT '',
    action           TEXT NOT NULL DEFAULT '',
    resource_digest  BYTEA,
    requirement      JSONB NOT NULL DEFAULT '{}'::jsonb,
    state            JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at       TIMESTAMPTZ NOT NULL,
    consumed_at      TIMESTAMPTZ,
    failed_attempts  INTEGER NOT NULL DEFAULT 0 CHECK (failed_attempts BETWEEN 0 AND 5),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (resource_digest IS NULL OR octet_length(resource_digest) = 32),
    CHECK (
        kind <> 'step_up'
        OR (session_id IS NOT NULL AND audience <> '' AND action <> '' AND resource_digest IS NOT NULL)
    )
);

CREATE INDEX authentication_transactions_expiry_idx
    ON authentication_transactions(expires_at) WHERE consumed_at IS NULL;
