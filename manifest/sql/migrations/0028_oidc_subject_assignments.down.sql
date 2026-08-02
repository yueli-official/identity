DROP TABLE oidc_subjects;
ALTER TABLE oidc_clients
    DROP COLUMN subject_sector,
    DROP COLUMN subject_type;
