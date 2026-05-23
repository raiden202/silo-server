DROP TABLE IF EXISTS media_group_overrides;
DROP TABLE IF EXISTS media_item_groups;
DROP TABLE IF EXISTS media_group_locations;
DROP TABLE IF EXISTS observed_media_locations;
DROP TABLE IF EXISTS scanned_media_groups;

DROP INDEX IF EXISTS idx_media_files_folder_observed_root;
DROP INDEX IF EXISTS idx_media_files_folder_group;

ALTER TABLE media_files
    DROP COLUMN IF EXISTS identity_json,
    DROP COLUMN IF EXISTS identity_confidence,
    DROP COLUMN IF EXISTS base_type,
    DROP COLUMN IF EXISTS base_year,
    DROP COLUMN IF EXISTS base_title,
    DROP COLUMN IF EXISTS group_key_version,
    DROP COLUMN IF EXISTS content_group_key,
    DROP COLUMN IF EXISTS observed_root_path;
