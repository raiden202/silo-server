DROP INDEX IF EXISTS idx_media_files_folder_file_modified_at;

ALTER TABLE media_files
    DROP COLUMN IF EXISTS file_modified_at;
