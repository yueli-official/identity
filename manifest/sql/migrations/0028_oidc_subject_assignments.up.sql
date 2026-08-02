-- OIDC subject assignment is explicit and persisted. First-party clients share
-- the public platform subject; third-party clients can opt into a pairwise
-- sector without exposing either the internal UUID or another sector's value.
ALTER TABLE oidc_clients
    ADD COLUMN subject_type   TEXT NOT NULL DEFAULT 'public'
        CHECK (subject_type IN ('public', 'pairwise')),
    ADD COLUMN subject_sector TEXT NOT NULL DEFAULT 'first-party';

CREATE TABLE oidc_subjects (
    identity_id UUID NOT NULL REFERENCES identities (id) ON DELETE CASCADE,
    sector_key  TEXT NOT NULL,
    subject     TEXT NOT NULL UNIQUE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('public', 'pairwise')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (identity_id, sector_key),
    CHECK (
        (subject_type = 'public' AND sector_key = 'public' AND subject ~ '^usr_[A-Za-z0-9_-]{22}$')
        OR
        (subject_type = 'pairwise' AND sector_key LIKE 'pairwise:%' AND subject ~ '^psu_[A-Za-z0-9_-]{22}$')
    )
);

INSERT INTO oidc_subjects (identity_id, sector_key, subject, subject_type)
SELECT id, 'public', user_key, 'public'
FROM identities;

CREATE INDEX idx_oidc_subjects_identity ON oidc_subjects (identity_id);
