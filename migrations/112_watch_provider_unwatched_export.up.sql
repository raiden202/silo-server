ALTER TABLE public.watch_provider_connections
    ADD COLUMN export_unwatched_enabled boolean NOT NULL DEFAULT false;
