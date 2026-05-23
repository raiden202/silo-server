ALTER TABLE public.media_folders
    DROP COLUMN IF EXISTS chapter_thumbnails_enabled;

ALTER TABLE public.media_files
    DROP COLUMN IF EXISTS chapters;
