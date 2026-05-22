ALTER TABLE platform_users DROP COLUMN IF EXISTS employee_id;
ALTER TABLE employees DROP COLUMN IF EXISTS merchant_role_id;
DROP TABLE IF EXISTS merchant_roles;
