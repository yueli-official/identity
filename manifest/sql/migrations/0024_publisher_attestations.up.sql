CREATE TABLE publisher_attestations (
    id                  UUID PRIMARY KEY,
    issuer              TEXT NOT NULL,
    publisher_subject   UUID NOT NULL,
    audience            TEXT NOT NULL,
    consumer_instance   TEXT NOT NULL,
    namespace           TEXT NOT NULL,
    artifact_kind       TEXT NOT NULL,
    artifact_identity   TEXT NOT NULL,
    artifact_version    TEXT NOT NULL,
    artifact_name       TEXT NOT NULL,
    artifact_uri        TEXT NOT NULL,
    artifact_media_type TEXT NOT NULL,
    artifact_sha256     CHAR(64) NOT NULL,
    statement_digest    CHAR(64) NOT NULL,
    command_digest      CHAR(64) NOT NULL,
    key_id              TEXT NOT NULL,
    envelope_json       JSONB NOT NULL,
    idempotency_key     TEXT NOT NULL,
    issued_at           TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT publisher_attestations_sha256_format
        CHECK (artifact_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT publisher_attestations_statement_digest_format
        CHECK (statement_digest ~ '^[0-9a-f]{64}$'),
    CONSTRAINT publisher_attestations_command_digest_format
        CHECK (command_digest ~ '^[0-9a-f]{64}$'),
    CONSTRAINT publisher_attestations_idempotency_unique
        UNIQUE (issuer, publisher_subject, idempotency_key),
    CONSTRAINT publisher_attestations_statement_unique
        UNIQUE (issuer, statement_digest)
);

CREATE INDEX publisher_attestations_subject_created_idx
    ON publisher_attestations (publisher_subject, created_at DESC);

CREATE INDEX publisher_attestations_target_idx
    ON publisher_attestations (
        audience,
        consumer_instance,
        namespace,
        artifact_kind,
        artifact_identity,
        artifact_version
    );
