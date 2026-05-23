ALTER TABLE public.media_files
    ADD COLUMN chapters jsonb;

ALTER TABLE public.media_folders
    ADD COLUMN chapter_thumbnails_enabled boolean NOT NULL DEFAULT false;
