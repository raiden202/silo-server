DROP INDEX IF EXISTS idx_series_root_match_queue_available;

CREATE INDEX IF NOT EXISTS idx_series_root_match_queue_attempt
    ON series_root_match_queue (last_attempted_at ASC NULLS FIRST, media_folder_id, observed_root_path);

ALTER TABLE series_root_match_queue
    DROP COLUMN IF EXISTS available_at;
