ALTER TABLE user_profiles
    ADD COLUMN is_primary BOOLEAN NOT NULL DEFAULT false;

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY user_id
               ORDER BY created_at ASC, id ASC
           ) AS rn
    FROM user_profiles
)
UPDATE user_profiles
SET is_primary = true
FROM ranked
WHERE user_profiles.id = ranked.id
  AND ranked.rn = 1;

CREATE UNIQUE INDEX user_profiles_primary_per_user
    ON user_profiles (user_id)
    WHERE is_primary;
