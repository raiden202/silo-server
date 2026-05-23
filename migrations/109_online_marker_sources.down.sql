DELETE FROM server_settings
WHERE key IN ('markers.mode', 'markers.lazy_playback');

ALTER TABLE media_files
    DROP COLUMN IF EXISTS credits_markers_detected_at,
    DROP COLUMN IF EXISTS credits_markers_algorithm,
    DROP COLUMN IF EXISTS credits_markers_confidence,
    DROP COLUMN IF EXISTS credits_markers_provider,
    DROP COLUMN IF EXISTS credits_markers_source,
    DROP COLUMN IF EXISTS intro_markers_provider;
