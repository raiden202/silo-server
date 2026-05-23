DROP INDEX IF EXISTS public.idx_media_files_subtitle_lang_gin;
DROP INDEX IF EXISTS public.idx_media_files_audio_lang_gin;
ALTER TABLE public.media_files DROP COLUMN IF EXISTS subtitle_language_codes;
ALTER TABLE public.media_files DROP COLUMN IF EXISTS audio_language_codes;
DROP FUNCTION IF EXISTS public.jsonb_track_language_codes(jsonb);
