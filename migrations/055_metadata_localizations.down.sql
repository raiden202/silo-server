DROP INDEX IF EXISTS idx_episode_localizations_language;
DROP INDEX IF EXISTS idx_season_localizations_language;
DROP INDEX IF EXISTS idx_media_item_localizations_language;

DROP TABLE IF EXISTS episode_localizations;
DROP TABLE IF EXISTS season_localizations;
DROP TABLE IF EXISTS media_item_localizations;

ALTER TABLE episodes
  DROP COLUMN IF EXISTS default_metadata_language;

ALTER TABLE seasons
  DROP COLUMN IF EXISTS default_metadata_language;

ALTER TABLE media_items
  DROP COLUMN IF EXISTS default_metadata_language;
