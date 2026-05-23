ALTER TABLE stock_flows DROP CONSTRAINT IF EXISTS stock_flows_type_check;
ALTER TABLE stock_flows ADD CONSTRAINT stock_flows_type_check CHECK (type IN ('sale', 'inbound', 'adjustment'));
ALTER TABLE stock_flows DROP COLUMN IF EXISTS operator_id;
ALTER TABLE stock_flows DROP COLUMN IF EXISTS operator_name;
ALTER TABLE stock_flows DROP COLUMN IF EXISTS notes;
ALTER TABLE stock_flows DROP COLUMN IF EXISTS warehouse_id;
ALTER TABLE stock_flows DROP COLUMN IF EXISTS related_flow_id;
DROP TABLE IF EXISTS warehouse_stocks;
DROP TABLE IF EXISTS warehouses;
