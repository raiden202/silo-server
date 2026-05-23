-- Replace any existing kometa collections with manual type.
UPDATE library_collections SET collection_type = 'manual' WHERE collection_type = 'kometa';

-- Drop old CHECK constraint and add updated one (kometa → tmdb).
ALTER TABLE library_collections DROP CONSTRAINT IF EXISTS library_collections_collection_type_check;
ALTER TABLE library_collections ADD CONSTRAINT library_collections_collection_type_check
  CHECK (collection_type = ANY (ARRAY['manual', 'mdblist', 'tmdb']));
