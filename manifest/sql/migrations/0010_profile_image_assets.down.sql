ALTER TABLE user_profiles
    DROP COLUMN IF EXISTS avatar_asset_id,
    DROP COLUMN IF EXISTS cover_asset_id;
