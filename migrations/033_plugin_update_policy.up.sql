ALTER TABLE plugin_installations
    ADD COLUMN update_policy TEXT NOT NULL DEFAULT 'auto',
    ADD COLUMN available_version TEXT;
