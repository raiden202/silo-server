\pset pager off

\echo '== Summary counts =='
WITH ids AS (
  SELECT 'tmdb' AS id_type, tmdb_id AS external_id
  FROM media_items
  WHERE type = 'series' AND tmdb_id <> ''

  UNION ALL

  SELECT 'tvdb' AS id_type, tvdb_id AS external_id
  FROM media_items
  WHERE type = 'series' AND tvdb_id <> ''

  UNION ALL

  SELECT 'imdb' AS id_type, imdb_id AS external_id
  FROM media_items
  WHERE type = 'series' AND imdb_id <> ''
),
duplicate_groups AS (
  SELECT id_type, external_id
  FROM ids
  GROUP BY id_type, external_id
  HAVING COUNT(*) > 1
),
file_hints AS (
  SELECT
    mf.content_id,
    MAX((regexp_match(mf.file_path, '\{tvdb-([0-9]+)\}'))[1]) AS path_tvdb_id,
    MAX((regexp_match(mf.file_path, '\{tmdb-([0-9]+)\}'))[1]) AS path_tmdb_id,
    MAX((regexp_match(mf.file_path, '\{imdb-(tt[0-9]+)\}'))[1]) AS path_imdb_id
  FROM media_files mf
  WHERE mf.content_id IS NOT NULL
  GROUP BY mf.content_id
),
mismatched_series AS (
  SELECT mi.content_id
  FROM file_hints fh
  JOIN media_items mi ON mi.content_id = fh.content_id
  WHERE mi.type = 'series'
    AND (
      (fh.path_tvdb_id IS NOT NULL AND mi.tvdb_id <> '' AND fh.path_tvdb_id <> mi.tvdb_id)
      OR (fh.path_tmdb_id IS NOT NULL AND mi.tmdb_id <> '' AND fh.path_tmdb_id <> mi.tmdb_id)
      OR (fh.path_imdb_id IS NOT NULL AND mi.imdb_id <> '' AND fh.path_imdb_id <> mi.imdb_id)
    )
),
season_counts AS (
  SELECT series_id
  FROM seasons
  GROUP BY series_id
),
episode_counts AS (
  SELECT series_id, COUNT(*) AS episode_count
  FROM episodes
  GROUP BY series_id
),
broken_series AS (
  SELECT mi.content_id
  FROM media_items mi
  JOIN season_counts sc ON sc.series_id = mi.content_id
  LEFT JOIN episode_counts ec ON ec.series_id = mi.content_id
  WHERE mi.type = 'series' AND ec.series_id IS NULL
),
broken_with_populated_owner AS (
  SELECT DISTINCT b.content_id
  FROM media_items b
  JOIN season_counts sc ON sc.series_id = b.content_id
  LEFT JOIN episode_counts ec_broken ON ec_broken.series_id = b.content_id
  JOIN media_items owners ON owners.type = 'series' AND owners.content_id <> b.content_id
  LEFT JOIN episode_counts ec_owner ON ec_owner.series_id = owners.content_id
  WHERE b.type = 'series'
    AND ec_broken.series_id IS NULL
    AND COALESCE(ec_owner.episode_count, 0) > 0
    AND (
      (b.tmdb_id <> '' AND owners.tmdb_id = b.tmdb_id)
      OR (b.tvdb_id <> '' AND owners.tvdb_id = b.tvdb_id)
      OR (b.imdb_id <> '' AND owners.imdb_id = b.imdb_id)
    )
)
SELECT 'duplicate_external_id_groups' AS metric, COUNT(*)::text AS value
FROM duplicate_groups
UNION ALL
SELECT 'folder_tag_mismatches', COUNT(*)::text
FROM mismatched_series
UNION ALL
SELECT 'series_with_seasons_but_zero_episodes', COUNT(*)::text
FROM broken_series
UNION ALL
SELECT 'broken_series_with_populated_duplicate_owner', COUNT(*)::text
FROM broken_with_populated_owner
ORDER BY metric;

\echo ''
\echo '== Duplicate series external IDs =='
WITH ids AS (
  SELECT content_id, title, year, 'tmdb' AS id_type, tmdb_id AS external_id
  FROM media_items
  WHERE type = 'series' AND tmdb_id <> ''

  UNION ALL

  SELECT content_id, title, year, 'tvdb' AS id_type, tvdb_id AS external_id
  FROM media_items
  WHERE type = 'series' AND tvdb_id <> ''

  UNION ALL

  SELECT content_id, title, year, 'imdb' AS id_type, imdb_id AS external_id
  FROM media_items
  WHERE type = 'series' AND imdb_id <> ''
),
dupes AS (
  SELECT id_type, external_id, COUNT(*) AS item_count
  FROM ids
  GROUP BY id_type, external_id
  HAVING COUNT(*) > 1
)
SELECT
  d.id_type,
  d.external_id,
  d.item_count,
  i.content_id,
  i.title,
  i.year
FROM dupes d
JOIN ids i USING (id_type, external_id)
ORDER BY d.item_count DESC, d.id_type, d.external_id, i.content_id;

\echo ''
\echo '== Folder-tag IDs that disagree with matched item IDs =='
WITH file_hints AS (
  SELECT
    mf.content_id,
    MAX((regexp_match(mf.file_path, '\{tvdb-([0-9]+)\}'))[1]) AS path_tvdb_id,
    MAX((regexp_match(mf.file_path, '\{tmdb-([0-9]+)\}'))[1]) AS path_tmdb_id,
    MAX((regexp_match(mf.file_path, '\{imdb-(tt[0-9]+)\}'))[1]) AS path_imdb_id,
    COUNT(*) AS file_count
  FROM media_files mf
  WHERE mf.content_id IS NOT NULL
  GROUP BY mf.content_id
)
SELECT
  mi.content_id,
  mi.title,
  mi.year,
  fh.path_tvdb_id,
  mi.tvdb_id,
  fh.path_tmdb_id,
  mi.tmdb_id,
  fh.path_imdb_id,
  mi.imdb_id,
  fh.file_count
FROM file_hints fh
JOIN media_items mi ON mi.content_id = fh.content_id
WHERE mi.type = 'series'
  AND (
    (fh.path_tvdb_id IS NOT NULL AND mi.tvdb_id <> '' AND fh.path_tvdb_id <> mi.tvdb_id)
    OR (fh.path_tmdb_id IS NOT NULL AND mi.tmdb_id <> '' AND fh.path_tmdb_id <> mi.tmdb_id)
    OR (fh.path_imdb_id IS NOT NULL AND mi.imdb_id <> '' AND fh.path_imdb_id <> mi.imdb_id)
  )
ORDER BY mi.title, mi.content_id;

\echo ''
\echo '== Series with seasons but zero episodes =='
WITH season_counts AS (
  SELECT series_id, COUNT(*) AS season_count
  FROM seasons
  GROUP BY series_id
),
episode_counts AS (
  SELECT series_id, COUNT(*) AS episode_count
  FROM episodes
  GROUP BY series_id
),
unlinked_files AS (
  SELECT content_id, COUNT(*) AS unlinked_series_files
  FROM media_files
  WHERE episode_id IS NULL AND missing_since IS NULL
  GROUP BY content_id
)
SELECT
  mi.content_id,
  mi.title,
  mi.year,
  mi.tmdb_id,
  mi.tvdb_id,
  mi.imdb_id,
  sc.season_count,
  COALESCE(uf.unlinked_series_files, 0) AS unlinked_series_files
FROM media_items mi
JOIN season_counts sc ON sc.series_id = mi.content_id
LEFT JOIN episode_counts ec ON ec.series_id = mi.content_id
LEFT JOIN unlinked_files uf ON uf.content_id = mi.content_id
WHERE mi.type = 'series' AND COALESCE(ec.episode_count, 0) = 0
ORDER BY sc.season_count DESC, COALESCE(uf.unlinked_series_files, 0) DESC, mi.title;

\echo ''
\echo '== Episode IMDb IDs claimed by more than one parent series =='
WITH episode_ids AS (
  SELECT
    e.imdb_id,
    e.series_id,
    mi.title AS series_title,
    COUNT(*) AS episode_rows
  FROM episodes e
  JOIN media_items mi ON mi.content_id = e.series_id
  WHERE e.imdb_id <> ''
  GROUP BY e.imdb_id, e.series_id, mi.title
),
dupes AS (
  SELECT imdb_id
  FROM episode_ids
  GROUP BY imdb_id
  HAVING COUNT(*) > 1
)
SELECT
  ei.imdb_id,
  ei.series_id,
  ei.series_title,
  ei.episode_rows
FROM dupes d
JOIN episode_ids ei USING (imdb_id)
ORDER BY ei.imdb_id, ei.series_id;

\echo ''
\echo '== Broken series that also share an external ID with another series =='
WITH season_counts AS (
  SELECT series_id
  FROM seasons
  GROUP BY series_id
),
episode_counts AS (
  SELECT series_id
  FROM episodes
  GROUP BY series_id
),
broken AS (
  SELECT mi.content_id
  FROM media_items mi
  JOIN season_counts sc ON sc.series_id = mi.content_id
  LEFT JOIN episode_counts ec ON ec.series_id = mi.content_id
  WHERE mi.type = 'series' AND ec.series_id IS NULL
),
ids AS (
  SELECT content_id, 'tmdb' AS id_type, tmdb_id AS external_id
  FROM media_items
  WHERE type = 'series' AND tmdb_id <> ''

  UNION ALL

  SELECT content_id, 'tvdb' AS id_type, tvdb_id AS external_id
  FROM media_items
  WHERE type = 'series' AND tvdb_id <> ''

  UNION ALL

  SELECT content_id, 'imdb' AS id_type, imdb_id AS external_id
  FROM media_items
  WHERE type = 'series' AND imdb_id <> ''
),
dupes AS (
  SELECT id_type, external_id
  FROM ids
  GROUP BY id_type, external_id
  HAVING COUNT(*) > 1
)
SELECT
  mi.content_id,
  mi.title,
  mi.year,
  i.id_type,
  i.external_id
FROM broken b
JOIN media_items mi ON mi.content_id = b.content_id
JOIN ids i ON i.content_id = b.content_id
JOIN dupes d USING (id_type, external_id)
ORDER BY mi.title, i.id_type;

\echo ''
\echo '== Broken series with populated duplicate-ID owner =='
WITH season_counts AS (
  SELECT series_id
  FROM seasons
  GROUP BY series_id
),
episode_counts AS (
  SELECT series_id, COUNT(*) AS episode_count
  FROM episodes
  GROUP BY series_id
),
broken AS (
  SELECT mi.content_id, mi.title, mi.year, mi.tmdb_id, mi.tvdb_id, mi.imdb_id
  FROM media_items mi
  JOIN season_counts sc ON sc.series_id = mi.content_id
  LEFT JOIN episode_counts ec ON ec.series_id = mi.content_id
  WHERE mi.type = 'series' AND COALESCE(ec.episode_count, 0) = 0
),
candidate_owners AS (
  SELECT
    b.content_id AS broken_content_id,
    b.title AS broken_title,
    b.year AS broken_year,
    owners.content_id AS owner_content_id,
    owners.title AS owner_title,
    owners.year AS owner_year,
    owners.tmdb_id,
    owners.tvdb_id,
    owners.imdb_id,
    ec.episode_count,
    CASE
      WHEN b.tmdb_id <> '' AND owners.tmdb_id = b.tmdb_id THEN 'tmdb'
      WHEN b.tvdb_id <> '' AND owners.tvdb_id = b.tvdb_id THEN 'tvdb'
      WHEN b.imdb_id <> '' AND owners.imdb_id = b.imdb_id THEN 'imdb'
    END AS matched_on
  FROM broken b
  JOIN media_items owners ON owners.type = 'series' AND owners.content_id <> b.content_id
  LEFT JOIN episode_counts ec ON ec.series_id = owners.content_id
  WHERE COALESCE(ec.episode_count, 0) > 0
    AND (
      (b.tmdb_id <> '' AND owners.tmdb_id = b.tmdb_id)
      OR (b.tvdb_id <> '' AND owners.tvdb_id = b.tvdb_id)
      OR (b.imdb_id <> '' AND owners.imdb_id = b.imdb_id)
    )
)
SELECT *
FROM candidate_owners
ORDER BY episode_count DESC, broken_title, owner_title;
