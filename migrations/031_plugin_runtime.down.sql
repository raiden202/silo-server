ALTER TABLE media_files
    DROP COLUMN markers_confidence,
    DROP COLUMN markers_source;

DROP INDEX IF EXISTS idx_media_analyzer_bindings_folder_installation;
DROP TABLE IF EXISTS media_analyzer_bindings;

DROP INDEX IF EXISTS idx_plugin_auth_identities_installation_subject;
DROP TABLE IF EXISTS plugin_auth_identities;

DROP INDEX IF EXISTS idx_plugin_auth_bindings_installation_capability;
DROP TABLE IF EXISTS plugin_auth_bindings;

DROP INDEX IF EXISTS idx_plugin_task_bindings_installation_capability;
DROP TABLE IF EXISTS plugin_task_bindings;

DROP INDEX IF EXISTS idx_plugin_runtime_configs_installation_key;
DROP TABLE IF EXISTS plugin_runtime_configs;

DROP INDEX IF EXISTS idx_plugin_capabilities_installation_capability;
DROP INDEX IF EXISTS idx_plugin_capabilities_installation;
DROP TABLE IF EXISTS plugin_capabilities;

DROP INDEX IF EXISTS idx_plugin_installations_plugin_version;
DROP TABLE IF EXISTS plugin_installations;

DROP INDEX IF EXISTS idx_plugin_repositories_enabled;
DROP TABLE IF EXISTS plugin_repositories;
