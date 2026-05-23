UPDATE server_settings
SET value = '24h'
WHERE key = 'jellyfin_compat.session_ttl'
  AND value = '87600h';
