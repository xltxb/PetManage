ALTER TABLE members ADD COLUMN IF NOT EXISTS phone_hash VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_members_phone_hash ON members(merchant_id, phone_hash);

ALTER TABLE employees ADD COLUMN IF NOT EXISTS phone_hash VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_employees_phone_hash ON employees(merchant_id, phone_hash);
