-- Remove the metadata_providers mirror table.
-- library_provider_chains now references plugin capabilities directly
-- via the composite natural key (plugin_installation_id, capability_id).

BEGIN;

-- Step 1: Add new columns (nullable initially for the data migration).
ALTER TABLE library_provider_chains
    ADD COLUMN plugin_installation_id BIGINT,
    ADD COLUMN capability_id TEXT,
    ADD COLUMN capability_type TEXT NOT NULL DEFAULT 'metadata_provider.v1';

-- Step 2: Populate from metadata_providers settings.
UPDATE library_provider_chains lpc
SET plugin_installation_id = (mp.settings->>'plugin_installation_id')::bigint,
    capability_id = mp.settings->>'capability_id'
FROM metadata_providers mp
WHERE lpc.provider_id = mp.id;

-- Step 3: Delete orphaned rows that couldn't be mapped (provider was
-- uninstalled but chain entry lingered). These are invalid anyway.
DELETE FROM library_provider_chains
WHERE plugin_installation_id IS NULL OR capability_id IS NULL;

-- Step 4: Make new columns NOT NULL now that data is populated.
ALTER TABLE library_provider_chains
    ALTER COLUMN plugin_installation_id SET NOT NULL,
    ALTER COLUMN capability_id SET NOT NULL;

-- Step 5: Drop old constraints and column.
ALTER TABLE library_provider_chains
    DROP CONSTRAINT library_provider_chains_provider_id_fkey;

ALTER TABLE library_provider_chains
    DROP CONSTRAINT library_provider_chains_pkey;

ALTER TABLE library_provider_chains
    DROP COLUMN provider_id;

-- Step 6: New PK and FK.
ALTER TABLE library_provider_chains
    ADD PRIMARY KEY (media_folder_id, plugin_installation_id, capability_id, content_level);

ALTER TABLE library_provider_chains
    ADD CONSTRAINT library_provider_chains_installation_fkey
    FOREIGN KEY (plugin_installation_id)
    REFERENCES plugin_installations(id) ON DELETE CASCADE;

-- Step 7: Drop the mirror table.
DROP TABLE IF EXISTS metadata_providers;
DROP SEQUENCE IF EXISTS metadata_providers_id_seq;

COMMIT;
