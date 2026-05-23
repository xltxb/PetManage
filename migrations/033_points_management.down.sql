DROP TABLE IF EXISTS point_transactions;
DROP TABLE IF EXISTS points_rules;
ALTER TABLE members DROP COLUMN IF EXISTS points_expire_at;
