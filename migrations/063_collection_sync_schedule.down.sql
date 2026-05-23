DROP INDEX IF EXISTS idx_library_collections_next_sync_due;

ALTER TABLE library_collections
    DROP COLUMN IF EXISTS next_sync_at,
    DROP COLUMN IF EXISTS sync_schedule;
