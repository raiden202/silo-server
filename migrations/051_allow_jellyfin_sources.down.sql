ALTER TABLE history_import_sources
    DROP CONSTRAINT history_import_sources_source_type_check,
    ADD CONSTRAINT history_import_sources_source_type_check
        CHECK (source_type IN ('emby', 'plex'));
