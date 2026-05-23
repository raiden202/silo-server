-- Merge collection_mode into collection_type for library collections.

-- Drop the old constraint first so the backfill can write 'smart'.
ALTER TABLE library_collections DROP CONSTRAINT library_collections_collection_type_check;

-- Backfill: smart mode → smart type.
UPDATE library_collections SET collection_type = 'smart' WHERE collection_mode = 'smart';

-- Drop the mode column.
ALTER TABLE library_collections DROP COLUMN collection_mode;

-- Re-add constraint with 'smart' included.
ALTER TABLE library_collections ADD CONSTRAINT library_collections_collection_type_check
  CHECK (collection_type = ANY (ARRAY['manual', 'smart', 'mdblist', 'tmdb']));

-- Rename collection_mode → collection_type for user personal collections.
ALTER TABLE user_personal_collections RENAME COLUMN collection_mode TO collection_type;
