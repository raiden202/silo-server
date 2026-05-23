ALTER TABLE public.watch_provider_scrobble_sessions
    DROP COLUMN IF EXISTS completed,
    DROP COLUMN IF EXISTS duration_seconds,
    DROP COLUMN IF EXISTS episode_number,
    DROP COLUMN IF EXISTS season_number,
    DROP COLUMN IF EXISTS series_tvdb_id,
    DROP COLUMN IF EXISTS series_tmdb_id,
    DROP COLUMN IF EXISTS series_imdb_id,
    DROP COLUMN IF EXISTS tvdb_id,
    DROP COLUMN IF EXISTS tmdb_id,
    DROP COLUMN IF EXISTS imdb_id,
    DROP COLUMN IF EXISTS kind,
    DROP COLUMN IF EXISTS provider_item_key;
