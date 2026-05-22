DROP INDEX IF EXISTS idx_employees_phone_hash;
ALTER TABLE employees DROP COLUMN IF EXISTS phone_hash;
DROP INDEX IF EXISTS idx_members_phone_hash;
ALTER TABLE members DROP COLUMN IF EXISTS phone_hash;
