ALTER TABLE public.media_files
    DROP COLUMN IF EXISTS chapter_thumbnail_last_error,
    DROP COLUMN IF EXISTS chapter_thumbnail_failure_count,
    DROP COLUMN IF EXISTS chapter_thumbnail_retry_after;
