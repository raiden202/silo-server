-- Restore the pre-127 normalization: collapse non-alnum to spaces and
-- lowercase, but DO NOT strip standalone "and" tokens. "&" and "and" will
-- once again be inequivalent in title matching.
--
-- The legacy idx_media_items_search (english combined title+overview) is
-- intentionally NOT restored: it was unused by the post-migration-105 query
-- path, and rolling back to a known-dead index just imposes a GIN rebuild
-- cost for no benefit.

DROP INDEX IF EXISTS public.idx_media_items_search_title_fields;
DROP INDEX IF EXISTS public.idx_media_items_title_normalized_trgm;

ALTER TABLE public.media_items DROP COLUMN IF EXISTS title_normalized;
ALTER TABLE public.media_items
  ADD COLUMN title_normalized text
  GENERATED ALWAYS AS (
    BTRIM(LOWER(REGEXP_REPLACE(COALESCE(title, ''), '[^[:alnum:]]+', ' ', 'g')))
  ) STORED;

CREATE INDEX IF NOT EXISTS idx_media_items_title_normalized_trgm
  ON public.media_items USING gin (title_normalized public.gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_media_items_search_title_fields
  ON public.media_items USING gin ((
    setweight(to_tsvector('simple', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('simple', COALESCE(original_title, '')), 'A') ||
    setweight(to_tsvector('simple', COALESCE(sort_title, '')), 'B')
  ));

DROP FUNCTION IF EXISTS public.normalize_search_text(text);
