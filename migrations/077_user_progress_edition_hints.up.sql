ALTER TABLE user_watch_progress
    ADD COLUMN IF NOT EXISTS last_edition_key TEXT;
