DROP INDEX IF EXISTS user_profiles_primary_per_user;

ALTER TABLE user_profiles
    DROP COLUMN IF EXISTS is_primary;
