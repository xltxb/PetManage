-- Rollback migration 049
DROP TABLE IF EXISTS fixed_expenses;
ALTER TABLE IF EXISTS order_items DROP COLUMN IF EXISTS cost_cents;
ALTER TABLE IF EXISTS service_items DROP COLUMN IF EXISTS cost_cents;
