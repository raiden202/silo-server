DROP INDEX IF EXISTS idx_user_collections_server_visible;

ALTER TABLE user_personal_collections
    DROP COLUMN IF EXISTS include_in_server_collections;
