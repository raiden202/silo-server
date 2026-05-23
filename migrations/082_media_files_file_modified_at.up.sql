ALTER TABLE media_files
    ADD COLUMN IF NOT EXISTS file_modified_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_media_files_folder_file_modified_at
    ON media_files (media_folder_id, file_modified_at);
