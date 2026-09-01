CREATE TABLE oidc_refresh_replay_receipts (
    key_digest TEXT PRIMARY KEY,
    client_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    response_ciphertext BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oidc_refresh_replay_receipts_expiry
    ON oidc_refresh_replay_receipts (expires_at);
CREATE INDEX idx_oidc_refresh_replay_receipts_request
    ON oidc_refresh_replay_receipts (request_id);

