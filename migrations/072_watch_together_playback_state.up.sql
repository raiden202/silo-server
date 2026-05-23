ALTER TABLE watch_together_rooms
    ADD COLUMN playback_state TEXT NOT NULL DEFAULT 'idle',
    ADD COLUMN resume_on_ready BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE watch_together_rooms
SET playback_state = CASE
        WHEN phase = 'playing' AND is_paused THEN 'paused'
        WHEN phase = 'playing' AND NOT is_paused THEN 'playing'
        ELSE 'idle'
    END,
    resume_on_ready = FALSE;

ALTER TABLE watch_together_rooms
    ADD CONSTRAINT watch_together_rooms_playback_state_check
        CHECK (playback_state IN ('idle', 'waiting', 'paused', 'playing'));
