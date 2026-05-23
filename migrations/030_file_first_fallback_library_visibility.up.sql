ALTER TABLE media_items
  ADD COLUMN episode_metadata_incomplete BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN episode_metadata_last_checked_at TIMESTAMPTZ;

ALTER TABLE seasons
  ADD COLUMN metadata_source TEXT NOT NULL DEFAULT 'provider';

ALTER TABLE episodes
  ADD COLUMN metadata_source TEXT NOT NULL DEFAULT 'provider';
