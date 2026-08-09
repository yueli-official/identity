-- Separate the internal UUIDv7, compact stable public key and mutable handle.
-- Migrations run before the only data source (devseed), so no SQL-side ID
-- generator or historical backfill is permitted.
ALTER TABLE identities
    ADD COLUMN user_key TEXT NOT NULL,
    ADD CONSTRAINT ck_identities_user_key
        CHECK (user_key ~ '^[1-9A-HJ-NP-Za-km-z]{8}$'),
    ADD CONSTRAINT uq_identities_user_key UNIQUE (user_key);

ALTER TABLE user_profiles ADD COLUMN handle TEXT;

-- Only carry forward unpublished fixture usernames that already satisfy the
-- new canonical contract. Invalid, reserved or duplicate fixture values become
-- unclaimed instead of creating a compatibility alias.
WITH normalized AS (
    SELECT
        identity_id,
        lower(btrim(username)) AS candidate,
        row_number() OVER (
            PARTITION BY lower(btrim(username))
            ORDER BY created_at, identity_id
        ) AS duplicate_rank
    FROM user_profiles
    WHERE username IS NOT NULL AND btrim(username) <> ''
)
UPDATE user_profiles AS profile
SET handle = normalized.candidate
FROM normalized
WHERE profile.identity_id = normalized.identity_id
  AND normalized.duplicate_rank = 1
  AND normalized.candidate ~ '^[a-z0-9][a-z0-9_]{1,28}[a-z0-9]$'
  AND normalized.candidate <> ALL (ARRAY[
      'about','account','admin','api','assets','auth','blog','callback','cdn',
      'docs','gallery','help','home','identity','login','logout','media','oauth',
      'oidc','privacy','profile','register','resource','security','settings',
      'shop','status','support','system','terms','user','users','www'
  ]);

DROP INDEX uq_user_profiles_username;
ALTER TABLE user_profiles DROP COLUMN username;
ALTER TABLE user_profiles
    ADD CONSTRAINT ck_user_profiles_handle
        CHECK (handle IS NULL OR handle ~ '^[a-z0-9][a-z0-9_]{1,28}[a-z0-9]$');
CREATE UNIQUE INDEX uq_user_profiles_handle
    ON user_profiles (handle) WHERE handle IS NOT NULL;

-- A row remains after rename or deletion, preventing a historical handle from
-- being reassigned to another user. One user has at most one current handle.
CREATE TABLE user_handle_history (
    handle      TEXT PRIMARY KEY,
    identity_id UUID NOT NULL REFERENCES identities (id) ON DELETE CASCADE,
    state       TEXT NOT NULL CHECK (state IN ('current', 'retired')),
    claimed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    retired_at  TIMESTAMPTZ
);

INSERT INTO user_handle_history (handle, identity_id, state)
SELECT handle, identity_id, 'current'
FROM user_profiles
WHERE handle IS NOT NULL;

CREATE UNIQUE INDEX uq_user_handle_history_current_identity
    ON user_handle_history (identity_id) WHERE state = 'current';
