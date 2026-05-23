DELETE FROM metadata_refresh_debt
WHERE target_type <> 'item';

ALTER TABLE metadata_refresh_debt
    DROP CONSTRAINT IF EXISTS metadata_refresh_debt_pkey;

ALTER TABLE metadata_refresh_debt
    DROP CONSTRAINT IF EXISTS metadata_refresh_debt_target_type_check;

ALTER TABLE metadata_refresh_debt
    ADD PRIMARY KEY (content_id);

ALTER TABLE metadata_refresh_debt
    ADD CONSTRAINT metadata_refresh_debt_content_id_fkey
        FOREIGN KEY (content_id) REFERENCES media_items(content_id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_metadata_refresh_debt_target_due;

ALTER TABLE metadata_refresh_debt
    DROP COLUMN IF EXISTS target_type;
