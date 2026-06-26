-- Extend user_profiles with the universal display fields that were previously
-- duplicated per consumer site (blog author_profiles): cover banner, bio, and
-- social links. The identity profile is the single source of truth for "who
-- this person is"; consumer sites read these via the public /profiles API.
ALTER TABLE user_profiles
    ADD COLUMN cover_url    TEXT,
    ADD COLUMN bio          TEXT,
    ADD COLUMN social_links JSONB NOT NULL DEFAULT '[]'::jsonb;
