DROP TABLE IF EXISTS public.watch_provider_favorite_items;

ALTER TABLE public.watch_provider_sync_runs
    DROP COLUMN IF EXISTS favorite_removals_sent,
    DROP COLUMN IF EXISTS outbound_favorites_sent,
    DROP COLUMN IF EXISTS outbound_favorites_found,
    DROP COLUMN IF EXISTS inbound_favorites_imported,
    DROP COLUMN IF EXISTS inbound_favorites_found;

ALTER TABLE public.watch_provider_connections
    DROP COLUMN IF EXISTS last_favorites_sync_at,
    DROP COLUMN IF EXISTS sync_favorite_removals_enabled,
    DROP COLUMN IF EXISTS export_favorites_enabled,
    DROP COLUMN IF EXISTS import_favorites_enabled;
