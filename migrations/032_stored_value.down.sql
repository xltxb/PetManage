ALTER TABLE members DROP COLUMN IF EXISTS bonus_balance_cents;
ALTER TABLE members DROP COLUMN IF EXISTS principal_balance_cents;
DROP TABLE IF EXISTS balance_transactions;
DROP TABLE IF EXISTS recharge_packages;
