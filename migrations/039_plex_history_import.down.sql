DROP TABLE IF EXISTS history_import_plex_sessions;

ALTER TABLE history_import_runs
    DROP CONSTRAINT history_import_runs_connection_mode_check,
    ADD CONSTRAINT history_import_runs_connection_mode_check
        CHECK (connection_mode IN ('connect', 'predefined', 'custom'));

ALTER TABLE history_import_runs
    DROP CONSTRAINT history_import_runs_source_type_check,
    ADD CONSTRAINT history_import_runs_source_type_check
        CHECK (source_type IN ('emby'));

ALTER TABLE history_import_sources
    DROP CONSTRAINT history_import_sources_source_type_check,
    ADD CONSTRAINT history_import_sources_source_type_check
        CHECK (source_type IN ('emby'));
