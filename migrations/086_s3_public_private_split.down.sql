DELETE FROM server_settings
WHERE key IN (
    's3.public_endpoint',
    's3.public_read_endpoint',
    's3.public_region',
    's3.public_path_style',
    's3.public_bucket',
    's3.public_key_prefix',
    's3.public_access_key',
    's3.public_secret_key',
    's3.public_url_auth',
    's3.public_token_secret',
    's3.public_token_param',
    's3.public_token_ttl',
    's3.private_endpoint',
    's3.private_region',
    's3.private_path_style',
    's3.private_bucket',
    's3.private_key_prefix',
    's3.private_access_key',
    's3.private_secret_key'
);
