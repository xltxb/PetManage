DROP TABLE IF EXISTS commission_records;
ALTER TABLE order_items DROP COLUMN IF EXISTS employee_id;
DROP TABLE IF EXISTS commission_rules;
