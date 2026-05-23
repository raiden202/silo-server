-- Rename column: encryption was removed; plain "admin_token" is clearer.
ALTER TABLE history_import_sources RENAME COLUMN encrypted_admin_token TO admin_token;
