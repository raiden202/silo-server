ALTER TABLE public.users
    DROP CONSTRAINT IF EXISTS users_max_profiles_min_check,
    ALTER COLUMN max_profiles SET DEFAULT 0;
