ALTER TABLE publisher_attestations ADD COLUMN publisher_subject_id UUID;
UPDATE publisher_attestations AS attestation
SET publisher_subject_id = identity.id
FROM identities AS identity
WHERE identity.user_key = attestation.publisher_subject;
DELETE FROM publisher_attestations WHERE publisher_subject_id IS NULL;

DROP INDEX publisher_attestations_subject_created_idx;
ALTER TABLE publisher_attestations
    DROP CONSTRAINT publisher_attestations_idempotency_unique,
    DROP CONSTRAINT publisher_attestations_subject_format,
    DROP COLUMN publisher_subject;
ALTER TABLE publisher_attestations RENAME COLUMN publisher_subject_id TO publisher_subject;
ALTER TABLE publisher_attestations
    ALTER COLUMN publisher_subject SET NOT NULL,
    ADD CONSTRAINT publisher_attestations_idempotency_unique
        UNIQUE (issuer, publisher_subject, idempotency_key);
CREATE INDEX publisher_attestations_subject_created_idx
    ON publisher_attestations (publisher_subject, created_at DESC);
