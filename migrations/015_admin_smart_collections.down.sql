UPDATE library_collections lc
SET library_id = lcl.library_id
FROM (
    SELECT DISTINCT ON (collection_id) collection_id, library_id
    FROM library_collection_libraries
    ORDER BY collection_id, library_id
) lcl
WHERE lc.id = lcl.collection_id;

DROP INDEX IF EXISTS idx_library_collection_libraries_library;
DROP TABLE IF EXISTS library_collection_libraries;

ALTER TABLE library_collections
    DROP COLUMN IF EXISTS sort_config,
    DROP COLUMN IF EXISTS query_definition,
    DROP COLUMN IF EXISTS collection_mode;
