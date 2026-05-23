ALTER TABLE watch_together_rooms
    DROP CONSTRAINT IF EXISTS watch_together_rooms_playback_state_check,
    DROP COLUMN IF EXISTS resume_on_ready,
    DROP COLUMN IF EXISTS playback_state;
