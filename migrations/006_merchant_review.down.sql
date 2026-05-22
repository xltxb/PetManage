-- Remove review columns from merchants.
ALTER TABLE merchants DROP COLUMN IF EXISTS review_remark;
ALTER TABLE merchants DROP COLUMN IF EXISTS reviewed_by;
ALTER TABLE merchants DROP COLUMN IF EXISTS reviewed_at;

-- Drop operation_logs table.
DROP TABLE IF EXISTS operation_logs;

-- Remove merchant_admin role.
DELETE FROM platform_roles WHERE code = 'merchant_admin';
