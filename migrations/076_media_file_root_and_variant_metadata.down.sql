DROP INDEX IF EXISTS idx_media_files_variant_group;
DROP INDEX IF EXISTS idx_media_files_folder_root;

ALTER TABLE media_files
    DROP COLUMN IF EXISTS multi_episode_end,
    DROP COLUMN IF EXISTS multi_episode_start,
    DROP COLUMN IF EXISTS presentation_part_total,
    DROP COLUMN IF EXISTS presentation_part_index,
    DROP COLUMN IF EXISTS presentation_group_key,
    DROP COLUMN IF EXISTS presentation_kind,
    DROP COLUMN IF EXISTS edition_source,
    DROP COLUMN IF EXISTS edition_confidence,
    DROP COLUMN IF EXISTS edition_key,
    DROP COLUMN IF EXISTS edition_raw,
    DROP COLUMN IF EXISTS canonical_root_path;
