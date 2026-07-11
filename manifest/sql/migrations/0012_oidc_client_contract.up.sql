ALTER TABLE oidc_clients
    ADD COLUMN secret_ref TEXT NOT NULL DEFAULT '',
    ADD COLUMN post_logout_redirect_uris TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN audiences TEXT[] NOT NULL DEFAULT '{}';

UPDATE oidc_clients
SET audiences = ARRAY[id]
WHERE cardinality(audiences) = 0;
