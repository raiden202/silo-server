DROP INDEX IF EXISTS public.idx_media_items_title_normalized_trgm;
ALTER TABLE public.media_items DROP COLUMN IF EXISTS title_normalized;
