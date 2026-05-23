DROP INDEX IF EXISTS idx_history_import_runs_running_heartbeat;

ALTER TABLE history_import_runs
    DROP COLUMN IF EXISTS last_heartbeat_at;
