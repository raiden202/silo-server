ALTER TABLE library_collections
    ADD COLUMN IF NOT EXISTS poster_auto_generated BOOLEAN NOT NULL DEFAULT FALSE;
