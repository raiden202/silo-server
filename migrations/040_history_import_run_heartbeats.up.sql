ALTER TABLE history_import_runs
    ADD COLUMN last_heartbeat_at timestamptz;

UPDATE history_import_runs
SET last_heartbeat_at = COALESCE(started_at, created_at)
WHERE status = 'running'
  AND last_heartbeat_at IS NULL;

CREATE INDEX idx_history_import_runs_running_heartbeat
    ON history_import_runs (status, last_heartbeat_at)
    WHERE status = 'running';
