DROP INDEX IF EXISTS public.idx_media_items_last_air_date_at;
ALTER TABLE public.media_items DROP COLUMN IF EXISTS last_air_date_at;
