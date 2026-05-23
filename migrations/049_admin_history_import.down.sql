ALTER TABLE history_import_runs
    DROP CONSTRAINT history_import_runs_connection_mode_check,
    ADD CONSTRAINT history_import_runs_connection_mode_check
        CHECK (connection_mode IN ('connect', 'predefined', 'custom', 'plex_oauth'));

DROP INDEX IF EXISTS idx_history_import_runs_mapping;

ALTER TABLE history_import_runs
    DROP COLUMN IF EXISTS mapping_id;

DROP INDEX IF EXISTS idx_history_import_user_mappings_user;
DROP INDEX IF EXISTS idx_history_import_user_mappings_source;

DROP TABLE IF EXISTS history_import_user_mappings;

ALTER TABLE history_import_sources
    DROP COLUMN IF EXISTS encrypted_admin_token;
