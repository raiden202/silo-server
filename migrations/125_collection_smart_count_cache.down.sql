DROP INDEX IF EXISTS idx_library_collections_smart_count_sort;

ALTER TABLE library_collections
    DROP COLUMN IF EXISTS item_count_cached_at,
    DROP COLUMN IF EXISTS item_count_cached;
