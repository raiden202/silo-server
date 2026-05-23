-- Allow personal user collections to act as TMDB/Trakt/MDBList imports.
--
-- Mirrors the source/sync columns we already keep on library_collections so a
-- profile-owned collection can store its source preset, last sync state, and a
-- cron schedule. Items continue to live in user_personal_collection_items and
-- are replaced by the user-collection sync service on each run.

ALTER TABLE user_personal_collections
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS sync_schedule TEXT,
    ADD COLUMN IF NOT EXISTS next_sync_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_sync_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_sync_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_sync_message TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS item_count INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_user_collections_next_sync_due
    ON user_personal_collections (next_sync_at)
    WHERE sync_schedule IS NOT NULL AND next_sync_at IS NOT NULL;
