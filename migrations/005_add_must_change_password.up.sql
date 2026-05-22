ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;

UPDATE platform_users SET must_change_password = true WHERE username = 'admin';
