ALTER TABLE public.watch_provider_connections
    ADD COLUMN sync_cursors jsonb NOT NULL DEFAULT '{}'::jsonb;
