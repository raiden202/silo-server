INSERT INTO server_settings (key, value)
VALUES ('scanner.empty_trash_after_scan', 'true')
ON CONFLICT (key) DO NOTHING;
