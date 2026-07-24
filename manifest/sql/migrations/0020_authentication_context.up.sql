-- Authentication events are immutable server-observed facts. Sessions point to
-- their current event; token issuance time must never replace authenticated_at.
CREATE TABLE authentication_events (
    id                    UUID PRIMARY KEY,
    identity_id           UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    session_id            UUID NOT NULL,
    authenticated_at      TIMESTAMPTZ NOT NULL,
    methods               TEXT[] NOT NULL DEFAULT '{}',
    factor_classes        TEXT[] NOT NULL DEFAULT '{}',
    assurance_level       TEXT NOT NULL CHECK (assurance_level IN ('unknown', 'aal1', 'aal2', 'aal3')),
    assurance_profile     TEXT NOT NULL,
    user_verified         BOOLEAN NOT NULL DEFAULT FALSE,
    phishing_resistant    BOOLEAN NOT NULL DEFAULT FALSE,
    recovery              BOOLEAN NOT NULL DEFAULT FALSE,
    credential_refs       TEXT[] NOT NULL DEFAULT '{}',
    policy_version        INTEGER NOT NULL CHECK (policy_version > 0),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX authentication_events_session_event_unique
    ON authentication_events(session_id, id);
CREATE INDEX authentication_events_identity_time_idx
    ON authentication_events(identity_id, authenticated_at DESC);

ALTER TABLE identity_sessions
    ADD COLUMN authentication_event_id UUID;

-- Existing sessions were authenticated before the context model existed. Keep
-- their real session creation time, but mark the method as legacy and do not
-- invent MFA, user verification, or phishing resistance.
INSERT INTO authentication_events (
    id, identity_id, session_id, authenticated_at, methods, factor_classes,
    assurance_level, assurance_profile, user_verified, phishing_resistant, recovery,
    credential_refs, policy_version
)
SELECT
    gen_random_uuid(), identity_id, id, created_at, ARRAY['legacy']::TEXT[], ARRAY[]::TEXT[],
    'aal1', 'urn:yueli:assurance:baseline', FALSE, FALSE, FALSE, ARRAY[]::TEXT[], 1
FROM identity_sessions;

UPDATE identity_sessions AS sessions
SET authentication_event_id = events.id
FROM authentication_events AS events
WHERE events.session_id = sessions.id;

ALTER TABLE identity_sessions
    ALTER COLUMN authentication_event_id SET NOT NULL,
    ADD CONSTRAINT identity_sessions_authentication_event_fk
        FOREIGN KEY (authentication_event_id)
        REFERENCES authentication_events(id);
