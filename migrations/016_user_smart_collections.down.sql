UPDATE user_personal_collections upc
SET profile_id = ucpp.profile_id
FROM (
    SELECT DISTINCT ON (user_id, collection_id) user_id, collection_id, profile_id
    FROM user_personal_collection_profiles
    ORDER BY user_id, collection_id, profile_id
) ucpp
WHERE upc.user_id = ucpp.user_id AND upc.id = ucpp.collection_id;

DROP INDEX IF EXISTS idx_user_collection_profiles_lookup;
DROP TABLE IF EXISTS user_personal_collection_profiles;

ALTER TABLE user_personal_collections
    DROP COLUMN IF EXISTS sort_config,
    DROP COLUMN IF EXISTS query_definition,
    DROP COLUMN IF EXISTS is_shared,
    DROP COLUMN IF EXISTS collection_mode,
    DROP COLUMN IF EXISTS creator_profile_id;
