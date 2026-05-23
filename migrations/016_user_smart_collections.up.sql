ALTER TABLE user_personal_collections
    ADD COLUMN creator_profile_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN collection_mode TEXT NOT NULL DEFAULT 'manual',
    ADD COLUMN is_shared BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN query_definition JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN sort_config JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE user_personal_collections
SET creator_profile_id = profile_id
WHERE creator_profile_id = '';

CREATE TABLE user_personal_collection_profiles (
    user_id INTEGER NOT NULL,
    collection_id TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    PRIMARY KEY (user_id, collection_id, profile_id),
    CONSTRAINT user_personal_collection_profiles_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

INSERT INTO user_personal_collection_profiles (user_id, collection_id, profile_id)
SELECT user_id, id, profile_id
FROM user_personal_collections
ON CONFLICT (user_id, collection_id, profile_id) DO NOTHING;

CREATE INDEX idx_user_collection_profiles_lookup
    ON user_personal_collection_profiles (user_id, profile_id, collection_id);
