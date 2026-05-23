-- Rollback canonicalization. Original code values were lossy on the way
-- down (multiple raw forms collapsed to one), so the data rewrite cannot
-- be reversed. This rollback restores the previous LOWER-only function
-- and re-creates the STORED columns; downstream UI will see whatever
-- canonical values the up migration left behind.

DROP INDEX IF EXISTS idx_media_files_audio_lang_gin;
DROP INDEX IF EXISTS idx_media_files_subtitle_lang_gin;

ALTER TABLE public.media_files DROP COLUMN IF EXISTS audio_language_codes;
ALTER TABLE public.media_files DROP COLUMN IF EXISTS subtitle_language_codes;

CREATE OR REPLACE FUNCTION public.jsonb_track_language_codes(tracks jsonb)
RETURNS text[]
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
AS $$
    SELECT array_agg(LOWER(elem->>'language'))
    FROM jsonb_array_elements(COALESCE(tracks, '[]'::jsonb)) AS elem
    WHERE (elem->>'language') IS NOT NULL AND (elem->>'language') <> ''
$$;

ALTER TABLE public.media_files
ADD COLUMN IF NOT EXISTS audio_language_codes text[]
  GENERATED ALWAYS AS (public.jsonb_track_language_codes(audio_tracks)) STORED;

ALTER TABLE public.media_files
ADD COLUMN IF NOT EXISTS subtitle_language_codes text[]
  GENERATED ALWAYS AS (public.jsonb_track_language_codes(subtitle_tracks)) STORED;

CREATE INDEX IF NOT EXISTS idx_media_files_audio_lang_gin
ON public.media_files USING gin (audio_language_codes);

CREATE INDEX IF NOT EXISTS idx_media_files_subtitle_lang_gin
ON public.media_files USING gin (subtitle_language_codes);

DROP FUNCTION IF EXISTS public.canonical_language_code(text);
