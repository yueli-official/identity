-- OIDC transient session persistence (milestone ④).
-- Generic table for short-lived authorize-code / PKCE / OIDC-connect sessions,
-- keyed by (kind, signature). The full fosite.Requester is serialized into data.
CREATE TABLE oidc_oauth_requests (
    kind       TEXT        NOT NULL,            -- 'authcode' | 'pkce' | 'oidc'
    signature  TEXT        NOT NULL,            -- fosite token/code signature
    request_id TEXT        NOT NULL,
    client_id  TEXT        NOT NULL,
    subject    TEXT        NOT NULL DEFAULT '',
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    data       JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, signature)
);

-- Refresh tokens carry security columns (spec §4): request_id is the rotation
-- family key; session_id binds the refresh to the IdP login session that minted
-- it (passive logout). access_signature links the access token issued alongside.
CREATE TABLE oidc_refresh_tokens (
    signature        TEXT        PRIMARY KEY,
    request_id       TEXT        NOT NULL,
    client_id        TEXT        NOT NULL,
    subject          TEXT        NOT NULL DEFAULT '',
    session_id       TEXT        NOT NULL DEFAULT '',
    access_signature TEXT        NOT NULL DEFAULT '',
    active           BOOLEAN     NOT NULL DEFAULT TRUE,
    expires_at       TIMESTAMPTZ,
    data             JSONB       NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_oidc_refresh_request   ON oidc_refresh_tokens (request_id);
CREATE INDEX idx_oidc_refresh_session   ON oidc_refresh_tokens (session_id);
CREATE INDEX idx_oidc_refresh_subject   ON oidc_refresh_tokens (subject);

-- Demo client gains the offline_access scope AND the refresh_token grant so the
-- SSO e2e can exercise refresh-token rotation (fosite rejects the refresh grant
-- unless the client lists refresh_token in grant_types).
UPDATE oidc_clients
   SET scopes      = '{openid,profile,email,roles,offline_access}',
       grant_types = '{authorization_code,refresh_token}'
 WHERE id = 'demo-web';
