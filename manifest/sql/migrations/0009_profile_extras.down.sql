ALTER TABLE user_profiles
    DROP COLUMN IF EXISTS cover_url,
    DROP COLUMN IF EXISTS bio,
    DROP COLUMN IF EXISTS social_links;
