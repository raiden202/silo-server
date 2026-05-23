-- Add content_level column to support per-level provider priorities.
-- Empty string means legacy flat chain (applies to all levels).
ALTER TABLE library_provider_chains
    ADD COLUMN content_level text NOT NULL DEFAULT '',
    ADD COLUMN enabled boolean NOT NULL DEFAULT true;

-- Drop old primary key and create new one including content_level.
ALTER TABLE library_provider_chains
    DROP CONSTRAINT library_provider_chains_pkey;

ALTER TABLE library_provider_chains
    ADD PRIMARY KEY (media_folder_id, provider_id, content_level);
