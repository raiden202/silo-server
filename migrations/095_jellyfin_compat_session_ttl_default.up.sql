UPDATE server_settings
SET value = '87600h'
WHERE key = 'jellyfin_compat.session_ttl'
  AND value = '24h';
