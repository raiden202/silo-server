DROP INDEX IF EXISTS idx_user_personal_collections_group_id_order;

ALTER TABLE user_personal_collections
    DROP CONSTRAINT IF EXISTS user_personal_collections_group_fkey;

UPDATE user_personal_collections upc
SET group_label = COALESCE(g.label, '')
FROM user_collection_groups g
WHERE g.user_id = upc.user_id
  AND g.id = upc.group_id;

ALTER TABLE user_personal_collections
    DROP COLUMN IF EXISTS group_id;

ALTER TABLE user_collection_groups
    DROP CONSTRAINT IF EXISTS user_collection_groups_user_slug_unique,
    DROP CONSTRAINT IF EXISTS user_collection_groups_user_id_unique,
    DROP CONSTRAINT IF EXISTS user_collection_groups_default_sort_mode_check,
    DROP COLUMN IF EXISTS default_sort_mode,
    DROP COLUMN IF EXISTS slug,
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS id;

ALTER TABLE media_folders
    DROP COLUMN IF EXISTS collection_ungrouped_sort_order;

DROP INDEX IF EXISTS idx_library_collection_libraries_group_order;

ALTER TABLE library_collection_libraries
    DROP CONSTRAINT IF EXISTS library_collection_libraries_group_library_fkey;

UPDATE library_collections lc
SET group_label = COALESCE(g.label, '')
FROM library_collection_libraries lcl
LEFT JOIN library_collection_groups g
    ON g.id = lcl.group_id
   AND g.library_id = lcl.library_id
WHERE lcl.collection_id = lc.id
  AND lc.library_id = lcl.library_id;

ALTER TABLE library_collection_libraries
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS sort_order,
    DROP COLUMN IF EXISTS group_id;

DROP INDEX IF EXISTS idx_library_collection_groups_order;
DROP INDEX IF EXISTS idx_library_collection_groups_user_unique;

DELETE FROM library_collection_groups
WHERE kind = 'user_collections'
   OR label = 'user-collections';

ALTER TABLE library_collection_groups
    DROP CONSTRAINT IF EXISTS library_collection_groups_library_slug_unique,
    DROP CONSTRAINT IF EXISTS library_collection_groups_id_library_unique,
    DROP CONSTRAINT IF EXISTS library_collection_groups_id_unique,
    DROP CONSTRAINT IF EXISTS library_collection_groups_default_sort_mode_check,
    DROP CONSTRAINT IF EXISTS library_collection_groups_kind_check,
    DROP COLUMN IF EXISTS default_sort_mode,
    DROP COLUMN IF EXISTS kind,
    DROP COLUMN IF EXISTS slug,
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS id;
