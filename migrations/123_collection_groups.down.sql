DROP INDEX IF EXISTS idx_user_personal_collections_group_order;

ALTER TABLE user_personal_collections DROP COLUMN IF EXISTS group_label;
ALTER TABLE library_collections DROP COLUMN IF EXISTS group_label;

DROP TABLE IF EXISTS user_collection_groups;
DROP TABLE IF EXISTS library_collection_groups;
