ALTER TABLE user_profiles ADD COLUMN username TEXT;
UPDATE user_profiles SET username = handle WHERE handle IS NOT NULL;
CREATE UNIQUE INDEX uq_user_profiles_username
    ON user_profiles (username) WHERE username IS NOT NULL;

DROP TABLE user_handle_history;
ALTER TABLE user_profiles DROP COLUMN handle;
ALTER TABLE identities DROP COLUMN user_key;
