-- Convert any tmdb collections to manual before reverting the constraint.
UPDATE library_collections SET collection_type = 'manual' WHERE collection_type = 'tmdb';

-- Revert CHECK constraint to pre-migration state (manual, mdblist, kometa).
ALTER TABLE library_collections DROP CONSTRAINT IF EXISTS library_collections_collection_type_check;
ALTER TABLE library_collections ADD CONSTRAINT library_collections_collection_type_check
  CHECK (collection_type = ANY (ARRAY['manual', 'mdblist', 'kometa']));
