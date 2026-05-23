ALTER TABLE public.watch_provider_scrobble_sessions
    ADD COLUMN IF NOT EXISTS provider_item_key text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS imdb_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS tmdb_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS tvdb_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS series_imdb_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS series_tmdb_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS series_tvdb_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS season_number integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS episode_number integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS duration_seconds double precision NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS completed boolean NOT NULL DEFAULT false;
