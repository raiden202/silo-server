DROP INDEX IF EXISTS public.idx_media_files_unmatched_match_queue;

ALTER TABLE public.media_files
    DROP COLUMN IF EXISTS match_attempted_at;
