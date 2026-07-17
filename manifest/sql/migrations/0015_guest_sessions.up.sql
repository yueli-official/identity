CREATE TABLE guest_sessions (
    id                  UUID PRIMARY KEY,
    token_hash          TEXT        NOT NULL UNIQUE,
    client_id           TEXT        NOT NULL REFERENCES oidc_clients(id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen           TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at          TIMESTAMPTZ NOT NULL,
    claimed_identity_id UUID        REFERENCES identities(id) ON DELETE SET NULL,
    claimed_at          TIMESTAMPTZ,
    revoked_at          TIMESTAMPTZ,
    CHECK ((claimed_identity_id IS NULL) = (claimed_at IS NULL))
);

CREATE INDEX idx_guest_sessions_client_expiry ON guest_sessions(client_id, expires_at);
CREATE INDEX idx_guest_sessions_claimed_identity ON guest_sessions(claimed_identity_id) WHERE claimed_identity_id IS NOT NULL;
