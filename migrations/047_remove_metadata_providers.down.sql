-- Reverse: recreate metadata_providers and remap chain entries back.
-- Note: display_name may not round-trip perfectly.

BEGIN;

-- Step 1: Recreate the metadata_providers table.
CREATE TABLE metadata_providers (
    id SERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    provider_type TEXT NOT NULL DEFAULT 'plugin',
    enabled BOOLEAN NOT NULL DEFAULT true,
    settings JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Step 2: Populate from plugin_capabilities.
INSERT INTO metadata_providers (slug, provider_type, settings)
SELECT
    pc.capability_id,
    'plugin',
    jsonb_build_object(
        'plugin_installation_id', pc.plugin_installation_id::text,
        'capability_id', pc.capability_id,
        'display_name', COALESCE(pc.metadata->>'display_name', pc.capability_id)
    )
FROM plugin_capabilities pc
WHERE pc.capability_type = 'metadata_provider.v1';

-- Step 3: Add provider_id column back to chain entries.
ALTER TABLE library_provider_chains
    ADD COLUMN provider_id INTEGER;

-- Step 4: Map composite key back to provider_id.
UPDATE library_provider_chains lpc
SET provider_id = mp.id
FROM metadata_providers mp
WHERE (mp.settings->>'plugin_installation_id')::bigint = lpc.plugin_installation_id
  AND mp.settings->>'capability_id' = lpc.capability_id;

-- Step 5: Delete any unmappable rows.
DELETE FROM library_provider_chains WHERE provider_id IS NULL;

ALTER TABLE library_provider_chains
    ALTER COLUMN provider_id SET NOT NULL;

-- Step 6: Drop composite key columns and constraints.
ALTER TABLE library_provider_chains
    DROP CONSTRAINT library_provider_chains_installation_fkey;

ALTER TABLE library_provider_chains
    DROP CONSTRAINT library_provider_chains_pkey;

ALTER TABLE library_provider_chains
    DROP COLUMN plugin_installation_id,
    DROP COLUMN capability_id,
    DROP COLUMN capability_type;

-- Step 7: Restore original PK and FK.
ALTER TABLE library_provider_chains
    ADD PRIMARY KEY (media_folder_id, provider_id, content_level);

ALTER TABLE library_provider_chains
    ADD CONSTRAINT library_provider_chains_provider_id_fkey
    FOREIGN KEY (provider_id) REFERENCES metadata_providers(id) ON DELETE CASCADE;

COMMIT;
