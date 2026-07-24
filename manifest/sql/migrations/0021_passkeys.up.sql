CREATE TABLE webauthn_users (
    identity_id UUID PRIMARY KEY REFERENCES identities(id) ON DELETE CASCADE,
    user_handle BYTEA NOT NULL UNIQUE CHECK (octet_length(user_handle) BETWEEN 32 AND 64),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webauthn_credentials (
    id                              UUID PRIMARY KEY,
    identity_id                     UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    rp_id                           TEXT NOT NULL,
    credential_id                   BYTEA NOT NULL,
    public_key                      BYTEA NOT NULL,
    public_key_algorithm            BIGINT NOT NULL,
    transports                      TEXT[] NOT NULL DEFAULT '{}',
    attachment                      TEXT NOT NULL DEFAULT '',
    attestation_type                TEXT NOT NULL DEFAULT '',
    attestation_format              TEXT NOT NULL DEFAULT '',
    aaguid                          BYTEA NOT NULL DEFAULT ''::bytea,
    sign_count                      BIGINT NOT NULL DEFAULT 0 CHECK (sign_count BETWEEN 0 AND 4294967295),
    clone_warning                   BOOLEAN NOT NULL DEFAULT FALSE,
    flags                           SMALLINT NOT NULL CHECK (flags BETWEEN 0 AND 255),
    user_verified_at_registration   BOOLEAN NOT NULL DEFAULT FALSE,
    backup_eligible                 BOOLEAN NOT NULL DEFAULT FALSE,
    backup_state                    BOOLEAN NOT NULL DEFAULT FALSE,
    attestation_client_data_json    BYTEA NOT NULL DEFAULT ''::bytea,
    attestation_client_data_hash    BYTEA NOT NULL DEFAULT ''::bytea,
    attestation_authenticator_data  BYTEA NOT NULL DEFAULT ''::bytea,
    attestation_object              BYTEA NOT NULL DEFAULT ''::bytea,
    status                          TEXT NOT NULL DEFAULT 'active'
                                    CHECK (status IN ('active', 'suspended', 'revoked')),
    label                           TEXT NOT NULL DEFAULT '',
    version                         BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at                    TIMESTAMPTZ,
    revoked_at                      TIMESTAMPTZ,
    revoked_reason                  TEXT NOT NULL DEFAULT '',
    UNIQUE (rp_id, credential_id)
);

CREATE INDEX webauthn_credentials_identity_status_idx
    ON webauthn_credentials(identity_id, status, created_at DESC);

CREATE TABLE authentication_ceremonies (
    id                UUID PRIMARY KEY,
    kind              TEXT NOT NULL
                      CHECK (kind IN ('passkey_registration', 'passkey_authentication')),
    identity_id       UUID REFERENCES identities(id) ON DELETE CASCADE,
    session_id        UUID,
    challenge_digest  BYTEA NOT NULL CHECK (octet_length(challenge_digest) = 32),
    library_state     JSONB NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    consumed_at       TIMESTAMPTZ,
    failed_attempts   INTEGER NOT NULL DEFAULT 0 CHECK (failed_attempts BETWEEN 0 AND 5),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (kind = 'passkey_registration' AND identity_id IS NOT NULL AND session_id IS NOT NULL)
        OR
        (kind = 'passkey_authentication' AND identity_id IS NULL AND session_id IS NULL)
    )
);

CREATE INDEX authentication_ceremonies_expiry_idx
    ON authentication_ceremonies(expires_at)
    WHERE consumed_at IS NULL;
