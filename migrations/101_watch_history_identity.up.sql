ALTER TABLE public.user_watch_history
    ADD COLUMN watch_identity JSONB NOT NULL DEFAULT '{}'::jsonb;
