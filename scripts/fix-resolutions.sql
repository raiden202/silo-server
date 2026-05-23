-- Fix resolution labels using upper-bound bucketing (Jellyfin-style).
-- Checks both width and height to correctly classify all aspect ratios.

UPDATE media_files
SET resolution = CASE
    WHEN (video_tracks->0->>'width')::int <= 854  AND (video_tracks->0->>'height')::int <= 480  THEN '480p'
    WHEN (video_tracks->0->>'width')::int <= 1280 AND (video_tracks->0->>'height')::int <= 962  THEN '720p'
    WHEN (video_tracks->0->>'width')::int <= 2560 AND (video_tracks->0->>'height')::int <= 1440 THEN '1080p'
    WHEN (video_tracks->0->>'width')::int <= 4096 AND (video_tracks->0->>'height')::int <= 3072 THEN '2160p'
    WHEN (video_tracks->0->>'width')::int <= 8192 AND (video_tracks->0->>'height')::int <= 6144 THEN '4320p'
    ELSE '2160p'
END
WHERE video_tracks IS NOT NULL
  AND jsonb_array_length(video_tracks) > 0
  AND resolution IS DISTINCT FROM CASE
    WHEN (video_tracks->0->>'width')::int <= 854  AND (video_tracks->0->>'height')::int <= 480  THEN '480p'
    WHEN (video_tracks->0->>'width')::int <= 1280 AND (video_tracks->0->>'height')::int <= 962  THEN '720p'
    WHEN (video_tracks->0->>'width')::int <= 2560 AND (video_tracks->0->>'height')::int <= 1440 THEN '1080p'
    WHEN (video_tracks->0->>'width')::int <= 4096 AND (video_tracks->0->>'height')::int <= 3072 THEN '2160p'
    WHEN (video_tracks->0->>'width')::int <= 8192 AND (video_tracks->0->>'height')::int <= 6144 THEN '4320p'
    ELSE '2160p'
END;
