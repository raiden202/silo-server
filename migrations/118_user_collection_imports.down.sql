DROP INDEX IF EXISTS idx_user_collections_next_sync_due;

ALTER TABLE user_personal_collections
    DROP COLUMN IF EXISTS item_count,
    DROP COLUMN IF EXISTS last_sync_message,
    DROP COLUMN IF EXISTS last_sync_status,
    DROP COLUMN IF EXISTS last_sync_at,
    DROP COLUMN IF EXISTS next_sync_at,
    DROP COLUMN IF EXISTS sync_schedule,
    DROP COLUMN IF EXISTS source_config,
    DROP COLUMN IF EXISTS source_url,
    DROP COLUMN IF EXISTS description;
