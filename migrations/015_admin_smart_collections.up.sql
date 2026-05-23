ALTER TABLE library_collections
    ADD COLUMN collection_mode TEXT NOT NULL DEFAULT 'manual',
    ADD COLUMN query_definition JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN sort_config JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE library_collection_libraries (
    collection_id TEXT NOT NULL,
    library_id INTEGER NOT NULL,
    PRIMARY KEY (collection_id, library_id),
    CONSTRAINT library_collection_libraries_collection_id_fkey
        FOREIGN KEY (collection_id) REFERENCES library_collections(id) ON DELETE CASCADE,
    CONSTRAINT library_collection_libraries_library_id_fkey
        FOREIGN KEY (library_id) REFERENCES media_folders(id) ON DELETE CASCADE
);

INSERT INTO library_collection_libraries (collection_id, library_id)
SELECT id, library_id
FROM library_collections
ON CONFLICT (collection_id, library_id) DO NOTHING;

CREATE INDEX idx_library_collection_libraries_library
    ON library_collection_libraries (library_id, collection_id);
