-- Remove per-level entries, keeping only legacy flat entries.
DELETE FROM library_provider_chains WHERE content_level != '';

ALTER TABLE library_provider_chains
    DROP CONSTRAINT library_provider_chains_pkey;

ALTER TABLE library_provider_chains
    ADD PRIMARY KEY (media_folder_id, provider_id);

ALTER TABLE library_provider_chains
    DROP COLUMN enabled,
    DROP COLUMN content_level;
