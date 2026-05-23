ALTER TABLE watch_together_rooms
    ADD COLUMN content_id TEXT,
    ADD COLUMN file_id INTEGER,
    ADD COLUMN library_id INTEGER,
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active';

UPDATE watch_together_rooms
SET content_id = COALESCE(selected_content_id, ''),
    file_id = selected_file_id,
    library_id = selected_library_id,
    status = CASE
        WHEN phase = 'ended' THEN 'closed'
        ELSE 'active'
    END;

ALTER TABLE watch_together_rooms
    ALTER COLUMN content_id SET NOT NULL,
    DROP CONSTRAINT IF EXISTS watch_together_rooms_phase_check,
    DROP CONSTRAINT IF EXISTS watch_together_rooms_selection_mode_check,
    DROP COLUMN IF EXISTS phase,
    DROP COLUMN IF EXISTS selection_mode,
    DROP COLUMN IF EXISTS selection_revision,
    DROP COLUMN IF EXISTS selected_content_id,
    DROP COLUMN IF EXISTS selected_file_id,
    DROP COLUMN IF EXISTS selected_library_id;

ALTER TABLE watch_together_rooms
    ADD CONSTRAINT watch_together_rooms_status_check
        CHECK (status IN ('active', 'closed'));

DROP INDEX IF EXISTS idx_watch_together_rooms_phase_created_at;
CREATE INDEX idx_watch_together_rooms_status_created_at
    ON watch_together_rooms (status, created_at DESC);
