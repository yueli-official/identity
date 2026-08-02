-- Profiles retain Asset ownership and public media keys, never a copied full
-- delivery URL. Rendition URLs are generated at the presentation boundary.
ALTER TABLE user_profiles
    ADD COLUMN avatar_media_key TEXT,
    ADD COLUMN cover_media_key  TEXT;

ALTER TABLE user_profiles
    ADD CONSTRAINT ck_user_profiles_avatar_media_key
        CHECK (avatar_media_key IS NULL OR avatar_media_key ~ '^[0-9A-Za-z]{20,32}$'),
    ADD CONSTRAINT ck_user_profiles_cover_media_key
        CHECK (cover_media_key IS NULL OR cover_media_key ~ '^[0-9A-Za-z]{20,32}$');

-- The service is unpublished: old copied URLs are deliberately discarded
-- instead of creating a dual-read compatibility contract.
ALTER TABLE user_profiles
    DROP COLUMN avatar_url,
    DROP COLUMN cover_url;
