-- Track the asset id behind avatar_url / cover_url so a replacement upload can
-- delete the previous asset (one avatar + one cover per user, no orphans).
ALTER TABLE user_profiles
    ADD COLUMN avatar_asset_id TEXT,
    ADD COLUMN cover_asset_id  TEXT;
