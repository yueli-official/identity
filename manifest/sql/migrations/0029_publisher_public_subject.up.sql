-- Publisher attestations are consumed outside Identity, so their stable subject
-- is the public user key rather than the internal UUID.
ALTER TABLE publisher_attestations ADD COLUMN publisher_subject_key TEXT;

UPDATE publisher_attestations AS attestation
SET publisher_subject_key = identity.user_key
FROM identities AS identity
WHERE identity.id = attestation.publisher_subject;

-- Unpublished fixtures without a live owning identity have no valid public
-- subject and are removed instead of retaining an internal-ID fallback.
DELETE FROM publisher_attestations WHERE publisher_subject_key IS NULL;

DROP INDEX publisher_attestations_subject_created_idx;
ALTER TABLE publisher_attestations
    DROP CONSTRAINT publisher_attestations_idempotency_unique,
    DROP COLUMN publisher_subject;
ALTER TABLE publisher_attestations RENAME COLUMN publisher_subject_key TO publisher_subject;
ALTER TABLE publisher_attestations
    ALTER COLUMN publisher_subject SET NOT NULL,
    ADD CONSTRAINT publisher_attestations_subject_format
        CHECK (publisher_subject ~ '^usr_[A-Za-z0-9_-]{22}$'),
    ADD CONSTRAINT publisher_attestations_idempotency_unique
        UNIQUE (issuer, publisher_subject, idempotency_key);
CREATE INDEX publisher_attestations_subject_created_idx
    ON publisher_attestations (publisher_subject, created_at DESC);
