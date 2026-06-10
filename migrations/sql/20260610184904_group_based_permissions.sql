-- +goose Up
-- +goose StatementBegin
CREATE TABLE groups (
    id                         serial PRIMARY KEY,
    slug                       text NOT NULL UNIQUE,
    name                       text NOT NULL,
    description                text NOT NULL DEFAULT '',
    built_in                   boolean NOT NULL DEFAULT false,
    permissions                text[] NOT NULL DEFAULT '{}',
    library_ids                integer[],
    max_streams                integer NOT NULL DEFAULT 6,
    max_transcodes             integer NOT NULL DEFAULT 2,
    max_profiles               integer NOT NULL DEFAULT 5,
    max_playback_quality       text NOT NULL DEFAULT '',
    download_allowed           boolean NOT NULL DEFAULT true,
    download_transcode_allowed boolean NOT NULL DEFAULT false,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_groups (
    user_id    integer NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id   integer NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, group_id)
);
CREATE INDEX user_groups_group_id_idx ON user_groups (group_id);

INSERT INTO groups (slug, name, description, built_in, permissions)
VALUES
  ('administrators', 'Administrators',
   'Full access to everything, including server administration.', true,
   ARRAY['admin']),
  ('users', 'Users',
   'Default group for new accounts.', true,
   ARRAY['marker_edit']);

-- Admins -> administrators (admin implies unrestricted policy).
INSERT INTO user_groups (user_id, group_id)
SELECT u.id, g.id
FROM users u
JOIN groups g ON g.slug = 'administrators'
WHERE u.role = 'admin';

-- Non-admin users whose policy deviates from the defaults: bucket by
-- distinct policy tuple, one group per tuple, members join ONLY that group
-- (joining 'users' too would erase restrictions via the permissive union).
WITH outliers AS (
    SELECT u.*
    FROM users u
    WHERE u.role IS DISTINCT FROM 'admin'
      AND (
           COALESCE(u.permissions, '{}') IS DISTINCT FROM ARRAY['marker_edit']::text[]
        OR u.library_ids IS NOT NULL
        OR u.max_streams IS DISTINCT FROM 6
        OR u.max_transcodes IS DISTINCT FROM 2
        OR u.max_profiles IS DISTINCT FROM 5
        OR COALESCE(u.max_playback_quality, '') IS DISTINCT FROM ''
        OR u.download_allowed IS DISTINCT FROM true
        OR u.download_transcode_allowed IS DISTINCT FROM false
      )
),
buckets AS (
    SELECT o.id AS user_id,
           dense_rank() OVER (
               ORDER BY o.permissions, o.library_ids, o.max_streams,
                        o.max_transcodes, o.max_profiles,
                        o.max_playback_quality, o.download_allowed,
                        o.download_transcode_allowed
           ) AS n,
           o.permissions, o.library_ids, o.max_streams, o.max_transcodes,
           o.max_profiles, o.max_playback_quality, o.download_allowed,
           o.download_transcode_allowed
    FROM outliers o
),
distinct_buckets AS (
    SELECT DISTINCT ON (n) n, permissions, library_ids, max_streams,
           max_transcodes, max_profiles, max_playback_quality,
           download_allowed, download_transcode_allowed
    FROM buckets
),
created AS (
    INSERT INTO groups (slug, name, description, built_in, permissions,
                        library_ids, max_streams, max_transcodes,
                        max_profiles, max_playback_quality, download_allowed,
                        download_transcode_allowed)
    SELECT 'migrated-policy-' || n,
           'Migrated policy ' || n,
           'Auto-created during the group migration to preserve a pre-existing per-user policy.',
           false,
           array_remove(COALESCE(permissions, '{}'), 'admin'),
           library_ids,
           COALESCE(max_streams, 6),
           COALESCE(max_transcodes, 2),
           COALESCE(max_profiles, 5),
           COALESCE(max_playback_quality, ''),
           COALESCE(download_allowed, true),
           COALESCE(download_transcode_allowed, false)
    FROM distinct_buckets
    RETURNING id, slug
)
INSERT INTO user_groups (user_id, group_id)
SELECT b.user_id, c.id
FROM buckets b
JOIN created c ON c.slug = 'migrated-policy-' || b.n;

-- Everyone not yet in any group (non-admin, default policy) -> users.
INSERT INTO user_groups (user_id, group_id)
SELECT u.id, g.id
FROM users u
JOIN groups g ON g.slug = 'users'
WHERE NOT EXISTS (SELECT 1 FROM user_groups ug WHERE ug.user_id = u.id);

ALTER TABLE users
    DROP COLUMN role,
    DROP COLUMN permissions,
    DROP COLUMN library_ids,
    DROP COLUMN max_streams,
    DROP COLUMN max_transcodes,
    DROP COLUMN max_profiles,
    DROP COLUMN max_playback_quality,
    DROP COLUMN download_allowed,
    DROP COLUMN download_transcode_allowed;

UPDATE users SET access_policy_revision = access_policy_revision + 1;

INSERT INTO server_settings (key, value)
VALUES ('users.default_group_slugs', '["users"]')
ON CONFLICT (key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN role text,
    ADD COLUMN permissions text[] NOT NULL DEFAULT '{}',
    ADD COLUMN library_ids integer[],
    ADD COLUMN max_streams integer NOT NULL DEFAULT 6,
    ADD COLUMN max_transcodes integer NOT NULL DEFAULT 2,
    ADD COLUMN max_profiles integer NOT NULL DEFAULT 5,
    ADD COLUMN max_playback_quality text NOT NULL DEFAULT '',
    ADD COLUMN download_allowed boolean NOT NULL DEFAULT true,
    ADD COLUMN download_transcode_allowed boolean NOT NULL DEFAULT false;

-- Best-effort flattening of effective policy back onto users.
UPDATE users u SET
    role = CASE WHEN EXISTS (
        SELECT 1 FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id
        WHERE ug.user_id = u.id AND 'admin' = ANY(g.permissions)
    ) THEN 'admin' ELSE 'user' END,
    permissions = COALESCE((
        SELECT array_agg(DISTINCT p ORDER BY p)
        FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id, unnest(g.permissions) AS p
        WHERE ug.user_id = u.id AND p <> 'admin'
    ), '{}'),
    library_ids = CASE WHEN EXISTS (
        SELECT 1 FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id
        WHERE ug.user_id = u.id
          AND (g.library_ids IS NULL OR 'admin' = ANY(g.permissions))
    ) THEN NULL ELSE (
        SELECT array_agg(DISTINCT lid ORDER BY lid)
        FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id, unnest(g.library_ids) AS lid
        WHERE ug.user_id = u.id
    ) END,
    max_streams = COALESCE((
        SELECT MAX(g.max_streams) FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id WHERE ug.user_id = u.id), 6),
    max_transcodes = COALESCE((
        SELECT MAX(g.max_transcodes) FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id WHERE ug.user_id = u.id), 2),
    max_profiles = COALESCE((
        SELECT MAX(g.max_profiles) FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id WHERE ug.user_id = u.id), 5),
    download_allowed = EXISTS (
        SELECT 1 FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id
        WHERE ug.user_id = u.id AND g.download_allowed),
    download_transcode_allowed = EXISTS (
        SELECT 1 FROM user_groups ug
        JOIN groups g ON g.id = ug.group_id
        WHERE ug.user_id = u.id AND g.download_transcode_allowed);

DELETE FROM server_settings WHERE key = 'users.default_group_slugs';
DROP TABLE user_groups;
DROP TABLE groups;

UPDATE users SET access_policy_revision = access_policy_revision + 1;
-- +goose StatementEnd
