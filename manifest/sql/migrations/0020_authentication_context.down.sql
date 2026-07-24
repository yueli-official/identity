ALTER TABLE identity_sessions
    DROP CONSTRAINT IF EXISTS identity_sessions_authentication_event_fk,
    DROP COLUMN IF EXISTS authentication_event_id;

DROP TABLE IF EXISTS authentication_events;
