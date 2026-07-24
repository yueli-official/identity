-- Consumer-owned authorization link for polling privacy requests. Foundation
-- deliberately omits subject identifiers from RightsRequestView.
CREATE TABLE identity_privacy_requests (
    request_id  TEXT PRIMARY KEY,
    identity_id UUID NOT NULL,
    status_token_hash TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX identity_privacy_requests_identity_idx
    ON identity_privacy_requests(identity_id, created_at DESC);
