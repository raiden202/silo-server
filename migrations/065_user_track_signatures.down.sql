ALTER TABLE user_audio_preferences
    DROP COLUMN IF EXISTS audio_track_signature;

ALTER TABLE user_subtitle_preferences
    DROP COLUMN IF EXISTS subtitle_track_signature;
