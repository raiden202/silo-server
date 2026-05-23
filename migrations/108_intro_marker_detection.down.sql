DROP TABLE IF EXISTS intro_season_analysis_state;
DROP INDEX IF EXISTS idx_media_intro_fingerprints_hash;
DROP TABLE IF EXISTS media_intro_fingerprints;

ALTER TABLE media_files
    DROP COLUMN IF EXISTS intro_markers_detected_at,
    DROP COLUMN IF EXISTS intro_markers_algorithm,
    DROP COLUMN IF EXISTS intro_markers_confidence,
    DROP COLUMN IF EXISTS intro_markers_source;

ALTER TABLE media_folders
    DROP COLUMN IF EXISTS intro_detection_enabled;
