DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM (
            SELECT provider, provider_id
            FROM media_item_provider_ids
            GROUP BY provider, provider_id
            HAVING COUNT(*) > 1
        ) duplicate_ids
    ) THEN
        RAISE EXCEPTION 'cross-type provider ids exist; resolve duplicates before rolling back migration 088';
    END IF;
END $$;

ALTER TABLE media_item_provider_ids
    DROP CONSTRAINT IF EXISTS media_item_provider_ids_provider_provider_id_item_type_key;

ALTER TABLE media_item_provider_ids
    DROP COLUMN IF EXISTS item_type;

ALTER TABLE media_item_provider_ids
    ADD CONSTRAINT media_item_provider_ids_provider_provider_id_key
    UNIQUE (provider, provider_id);

CREATE INDEX IF NOT EXISTS idx_media_item_provider_ids_provider_id
    ON media_item_provider_ids (provider, provider_id);
