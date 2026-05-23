-- Recreate dropped duplicate indexes (matching original 001 schema)
CREATE INDEX IF NOT EXISTS idx_api_keys_api_key
ON public.api_keys USING btree (api_key);

CREATE INDEX IF NOT EXISTS idx_seasons_series
ON public.seasons USING btree (series_id, season_number);

CREATE INDEX IF NOT EXISTS idx_episodes_series
ON public.episodes USING btree (series_id, season_number, episode_number);

-- Drop new indexes
DROP INDEX IF EXISTS public.idx_item_people_person_kind;
DROP INDEX IF EXISTS public.idx_uwp_profile_in_progress;
DROP INDEX IF EXISTS public.idx_uwp_profile_completed;
DROP INDEX IF EXISTS public.idx_media_items_keywords;
DROP INDEX IF EXISTS public.idx_media_items_tvdb_id;
DROP INDEX IF EXISTS public.idx_media_items_imdb_id;
DROP INDEX IF EXISTS public.idx_media_items_tmdb_id;
DROP INDEX IF EXISTS public.idx_media_items_release_date;
DROP INDEX IF EXISTS public.idx_media_items_sort_key;
DROP INDEX IF EXISTS public.idx_people_name_trgm;
-- idx_people_name_lower is owned by migration 007; do not drop on 102 rollback.
-- Note: pg_trgm extension is left installed; harmless if unused.
