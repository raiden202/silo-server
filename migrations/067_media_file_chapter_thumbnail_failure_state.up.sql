ALTER TABLE public.media_files
    ADD COLUMN chapter_thumbnail_retry_after timestamptz,
    ADD COLUMN chapter_thumbnail_failure_count integer NOT NULL DEFAULT 0,
    ADD COLUMN chapter_thumbnail_last_error text;
