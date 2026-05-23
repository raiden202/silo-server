ALTER TABLE user_audio_preferences
    ADD COLUMN IF NOT EXISTS audio_track_signature JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE user_subtitle_preferences
    ADD COLUMN IF NOT EXISTS subtitle_track_signature JSONB NOT NULL DEFAULT '{}'::jsonb;
