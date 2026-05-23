ALTER TABLE user_personal_collections
    DROP COLUMN IF EXISTS poster_thumbhash,
    DROP COLUMN IF EXISTS poster_url;
