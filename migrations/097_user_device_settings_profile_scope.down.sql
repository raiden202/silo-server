ALTER TABLE public.user_device_settings RENAME TO user_device_settings_profile_scoped_legacy;

CREATE TABLE IF NOT EXISTS public.user_device_settings_legacy_restore (
    user_id integer NOT NULL,
    device_id text NOT NULL,
    key text NOT NULL,
    value text NOT NULL,
    device_name text NOT NULL DEFAULT '',
    device_platform text NOT NULL DEFAULT '',
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_device_settings_legacy_restore_pkey PRIMARY KEY (user_id, device_id, key),
    CONSTRAINT user_device_settings_legacy_restore_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE
);

INSERT INTO public.user_device_settings_legacy_restore (
    user_id,
    device_id,
    key,
    value,
    device_name,
    device_platform,
    updated_at
)
SELECT DISTINCT ON (user_id, device_id, key)
    user_id,
    device_id,
    key,
    value,
    device_name,
    device_platform,
    updated_at
FROM public.user_device_settings_profile_scoped_legacy
ORDER BY user_id, device_id, key, updated_at DESC, profile_id ASC;

CREATE INDEX IF NOT EXISTS idx_user_device_settings_legacy_restore_user_key
    ON public.user_device_settings_legacy_restore (user_id, key);

DROP TABLE public.user_device_settings_profile_scoped_legacy;

ALTER TABLE public.user_device_settings_legacy_restore RENAME TO user_device_settings;
ALTER INDEX public.idx_user_device_settings_legacy_restore_user_key RENAME TO idx_user_device_settings_user_key;
