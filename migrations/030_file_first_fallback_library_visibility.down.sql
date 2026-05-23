ALTER TABLE episodes
  DROP COLUMN metadata_source;

ALTER TABLE seasons
  DROP COLUMN metadata_source;

ALTER TABLE media_items
  DROP COLUMN episode_metadata_last_checked_at,
  DROP COLUMN episode_metadata_incomplete;
