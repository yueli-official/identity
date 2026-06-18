-- 0006_roles.up.sql
-- Coarse-grained RBAC: a small role catalog + an identity↔role join. The roles
-- claim in issued tokens is the set of slugs for the identity. No policy/Casbin.
CREATE TABLE roles (
    slug        TEXT        PRIMARY KEY,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO roles (slug, description) VALUES
    ('user',  'Standard account'),
    ('admin', 'Administrator');

CREATE TABLE identity_roles (
    identity_id UUID        NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    role_slug   TEXT        NOT NULL REFERENCES roles(slug),
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (identity_id, role_slug)
);
CREATE INDEX idx_identity_roles_identity ON identity_roles (identity_id);
