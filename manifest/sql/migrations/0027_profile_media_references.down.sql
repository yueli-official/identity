ALTER TABLE user_profiles
    ADD COLUMN avatar_url TEXT,
    ADD COLUMN cover_url  TEXT;

ALTER TABLE user_profiles
    DROP COLUMN avatar_media_key,
    DROP COLUMN cover_media_key;
