-- Seed a default platform admin user (admin / admin123).
-- The bcrypt hash below corresponds to "admin123".
INSERT INTO platform_roles (code, name, permissions) VALUES ('super_admin', 'Super Admin', '["*"]') ON CONFLICT (code) DO NOTHING;

INSERT INTO platform_users (username, password_hash, display_name, role_id, status)
SELECT 'admin', '$2a$10$n/PJYYTdH7G5c7j1Cj5tcOuzuPAmuyaRr1Ni1mcUZAH3hZG97M82C', 'Super Admin', r.id, 'active'
FROM platform_roles r
WHERE r.code = 'super_admin'
AND NOT EXISTS (SELECT 1 FROM platform_users WHERE username = 'admin');
