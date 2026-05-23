INSERT INTO server_settings (key, value)
SELECT 's3.public_endpoint', value FROM server_settings WHERE key = 's3.operational_endpoint'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.private_endpoint', value FROM server_settings WHERE key = 's3.operational_endpoint'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_region', value FROM server_settings WHERE key = 's3.operational_region'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.private_region', value FROM server_settings WHERE key = 's3.operational_region'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_path_style', value FROM server_settings WHERE key = 's3.operational_path_style'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.private_path_style', value FROM server_settings WHERE key = 's3.operational_path_style'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_bucket', value FROM server_settings WHERE key = 's3.operational_bucket'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.private_bucket', value FROM server_settings WHERE key = 's3.operational_bucket'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_key_prefix', value FROM server_settings WHERE key = 's3.operational_key_prefix'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.private_key_prefix', value FROM server_settings WHERE key = 's3.operational_key_prefix'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_access_key', value FROM server_settings WHERE key = 's3.operational_access_key'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.private_access_key', value FROM server_settings WHERE key = 's3.operational_access_key'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_secret_key', value FROM server_settings WHERE key = 's3.operational_secret_key'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.private_secret_key', value FROM server_settings WHERE key = 's3.operational_secret_key'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_read_endpoint', value FROM server_settings WHERE key = 's3.operational_public_endpoint'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_url_auth', value FROM server_settings WHERE key = 's3.operational_url_auth'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_token_secret', value FROM server_settings WHERE key = 's3.operational_token_secret'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_token_param', value FROM server_settings WHERE key = 's3.operational_token_param'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO server_settings (key, value)
SELECT 's3.public_token_ttl', value FROM server_settings WHERE key = 's3.operational_token_ttl'
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
