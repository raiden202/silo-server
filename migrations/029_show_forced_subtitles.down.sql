ALTER TABLE user_subtitle_preferences
  DROP COLUMN show_forced_subtitles;

ALTER TABLE user_library_playback_preferences
  DROP COLUMN show_forced_subtitles;

ALTER TABLE user_profiles
  DROP COLUMN show_forced_subtitles;
