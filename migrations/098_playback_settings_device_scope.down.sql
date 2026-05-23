INSERT INTO public.user_settings (
    user_id,
    key,
    value
)
SELECT DISTINCT ON (user_id, key)
    user_id,
    key,
    value
FROM public.user_device_settings
WHERE key IN (
    'playback.preferred_quality',
    'playback.audio_language',
    'playback.auto_skip_intro',
    'playback.auto_skip_credits',
    'playback.auto_play_next'
)
ORDER BY user_id, key, updated_at DESC, profile_id ASC, device_id ASC
ON CONFLICT (user_id, key) DO NOTHING;
