-- Node heartbeat table for detecting crashed nodes.
CREATE TABLE IF NOT EXISTS node_heartbeats (
    node_id    TEXT PRIMARY KEY,
    node_type  TEXT NOT NULL,
    node_url   TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Track pause state and WebSocket presence on synced sessions.
ALTER TABLE playback_sessions_sync
    ADD COLUMN IF NOT EXISTS is_paused BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE playback_sessions_sync
    ADD COLUMN IF NOT EXISTS has_websocket BOOLEAN NOT NULL DEFAULT FALSE;
