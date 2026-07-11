ALTER TABLE oidc_clients
    DROP COLUMN audiences,
    DROP COLUMN post_logout_redirect_uris,
    DROP COLUMN secret_ref;
