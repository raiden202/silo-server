ALTER TABLE playback_sessions_sync DROP COLUMN IF EXISTS has_websocket;
ALTER TABLE playback_sessions_sync DROP COLUMN IF EXISTS is_paused;
DROP TABLE IF EXISTS node_heartbeats;
