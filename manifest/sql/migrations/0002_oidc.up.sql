-- OIDC tables (milestone 3). Transient OAuth sessions (codes/pkce/access/oidc)
-- live in fosite's in-memory store in milestone 3; they get PG/Redis persistence in milestone 4.
CREATE TABLE oidc_clients (
    id             TEXT PRIMARY KEY,
    secret_hash    TEXT NOT NULL DEFAULT '',
    public         BOOLEAN NOT NULL DEFAULT TRUE,
    redirect_uris  TEXT[] NOT NULL DEFAULT '{}',
    grant_types    TEXT[] NOT NULL DEFAULT '{authorization_code}',
    response_types TEXT[] NOT NULL DEFAULT '{code}',
    scopes         TEXT[] NOT NULL DEFAULT '{openid,profile,email,roles}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE oidc_signing_keys (
    kid         TEXT PRIMARY KEY,
    alg         TEXT NOT NULL DEFAULT 'RS256',
    private_pem TEXT NOT NULL,
    public_pem  TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active', 'next', 'retired')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    retired_at  TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_oidc_signing_keys_active
    ON oidc_signing_keys ((status)) WHERE status = 'active';

-- Demo public client for SSO e2e / local development (first-party, PKCE).
INSERT INTO oidc_clients (id, public, redirect_uris, grant_types, response_types, scopes)
VALUES (
    'demo-web', TRUE,
    '{http://127.0.0.1:3000/callback,http://localhost:3000/callback}',
    '{authorization_code}', '{code}', '{openid,profile,email,roles}'
)
ON CONFLICT (id) DO NOTHING;
