ALTER TABLE watch_together_rooms
    ADD COLUMN phase TEXT NOT NULL DEFAULT 'lobby',
    ADD COLUMN selection_mode TEXT NOT NULL DEFAULT 'host_pick',
    ADD COLUMN selection_revision BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN selected_content_id TEXT,
    ADD COLUMN selected_file_id INTEGER,
    ADD COLUMN selected_library_id INTEGER;

UPDATE watch_together_rooms
SET selected_content_id = content_id,
    selected_file_id = file_id,
    selected_library_id = library_id,
    phase = CASE
        WHEN content_id IS NOT NULL AND BTRIM(content_id) <> '' THEN 'playing'
        ELSE 'lobby'
    END,
    selection_mode = 'host_pick',
    selection_revision = CASE
        WHEN content_id IS NOT NULL AND BTRIM(content_id) <> '' THEN 1
        ELSE 0
    END;

ALTER TABLE watch_together_rooms
    DROP CONSTRAINT IF EXISTS watch_together_rooms_status_check,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS content_id,
    DROP COLUMN IF EXISTS file_id,
    DROP COLUMN IF EXISTS library_id;

ALTER TABLE watch_together_rooms
    ADD CONSTRAINT watch_together_rooms_phase_check
        CHECK (phase IN ('lobby', 'playing', 'ended')),
    ADD CONSTRAINT watch_together_rooms_selection_mode_check
        CHECK (selection_mode IN ('host_pick', 'vote'));

DROP INDEX IF EXISTS idx_watch_together_rooms_status_created_at;
CREATE INDEX idx_watch_together_rooms_phase_created_at
    ON watch_together_rooms (phase, created_at DESC);
