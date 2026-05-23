DROP INDEX IF EXISTS idx_library_collections_section_management_key;

ALTER TABLE library_collections DROP CONSTRAINT IF EXISTS library_collections_management_mode_check;

ALTER TABLE library_collections
  DROP COLUMN IF EXISTS management_key,
  DROP COLUMN IF EXISTS management_source,
  DROP COLUMN IF EXISTS management_mode;
