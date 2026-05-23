ALTER TABLE media_files
    ADD COLUMN IF NOT EXISTS intro_markers_provider text,
    ADD COLUMN IF NOT EXISTS credits_markers_source text,
    ADD COLUMN IF NOT EXISTS credits_markers_provider text,
    ADD COLUMN IF NOT EXISTS credits_markers_confidence double precision,
    ADD COLUMN IF NOT EXISTS credits_markers_algorithm text,
    ADD COLUMN IF NOT EXISTS credits_markers_detected_at timestamp with time zone;

INSERT INTO server_settings (key, value) VALUES ('markers.mode', 'local')
ON CONFLICT (key) DO NOTHING;

INSERT INTO server_settings (key, value) VALUES ('markers.lazy_playback', 'false')
ON CONFLICT (key) DO NOTHING;
