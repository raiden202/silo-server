ALTER TABLE library_collections DROP CONSTRAINT IF EXISTS library_collections_collection_type_check;

ALTER TABLE library_collections ADD CONSTRAINT library_collections_collection_type_check
  CHECK (collection_type = ANY (ARRAY['manual', 'smart', 'mdblist', 'tmdb', 'trakt']));

