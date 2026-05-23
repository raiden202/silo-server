ALTER TABLE user_profiles
  ADD COLUMN show_forced_subtitles BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE user_library_playback_preferences
  ADD COLUMN show_forced_subtitles BOOLEAN;

ALTER TABLE user_subtitle_preferences
  ADD COLUMN show_forced_subtitles BOOLEAN;
