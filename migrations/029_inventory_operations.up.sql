-- 029: inventory operations — warehouses, warehouse_stocks, expanded stock_flows
CREATE TABLE IF NOT EXISTS warehouses (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(255) NOT NULL,
    address VARCHAR(500) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_warehouses_merchant_id ON warehouses(merchant_id);

CREATE TABLE IF NOT EXISTS warehouse_stocks (
    id BIGSERIAL PRIMARY KEY,
    warehouse_id BIGINT NOT NULL REFERENCES warehouses(id),
    product_id BIGINT NOT NULL REFERENCES products(id),
    product_sku_id BIGINT,
    stock INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_warehouse_stocks_unique ON warehouse_stocks(warehouse_id, product_id, COALESCE(product_sku_id, 0));
CREATE INDEX idx_warehouse_stocks_product_id ON warehouse_stocks(product_id);

-- Expand stock_flows type and add new columns
ALTER TABLE stock_flows
    ADD COLUMN IF NOT EXISTS operator_id BIGINT,
    ADD COLUMN IF NOT EXISTS operator_name VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS notes VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS warehouse_id BIGINT REFERENCES warehouses(id),
    ADD COLUMN IF NOT EXISTS related_flow_id BIGINT;

-- Drop old CHECK and add expanded one
ALTER TABLE stock_flows DROP CONSTRAINT IF EXISTS stock_flows_type_check;
ALTER TABLE stock_flows ADD CONSTRAINT stock_flows_type_check CHECK (type IN ('sale', 'inbound', 'outbound', 'transfer_out', 'transfer_in', 'loss', 'surplus', 'adjustment'));

CREATE INDEX IF NOT EXISTS idx_stock_flows_warehouse_id ON stock_flows(warehouse_id);
CREATE INDEX IF NOT EXISTS idx_stock_flows_type ON stock_flows(merchant_id, type);
CREATE INDEX IF NOT EXISTS idx_stock_flows_operator_id ON stock_flows(operator_id);
