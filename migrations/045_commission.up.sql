-- 045: Employee commission management
-- commission_rules: per-merchant commission rate configuration
-- commission_records: commission earned by employees per order item

CREATE TABLE IF NOT EXISTS commission_rules (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    product_commission_rate NUMERIC(5,2) NOT NULL DEFAULT 5.00,
    service_commission_rate NUMERIC(5,2) NOT NULL DEFAULT 30.00,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_commission_rules_merchant
    ON commission_rules (merchant_id);

-- Add employee_id to order_items to track the servicing technician
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS employee_id BIGINT REFERENCES employees(id);

CREATE TABLE IF NOT EXISTS commission_records (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    order_id BIGINT NOT NULL REFERENCES orders(id),
    order_item_id BIGINT NOT NULL REFERENCES order_items(id),
    item_type VARCHAR(20) NOT NULL CHECK (item_type IN ('product', 'service')),
    order_item_amount_cents INT NOT NULL,
    commission_rate NUMERIC(5,2) NOT NULL,
    commission_cents INT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'confirmed'
        CHECK (status IN ('confirmed', 'deducted')),
    refund_id BIGINT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_commission_records_merchant
    ON commission_records (merchant_id, created_at);

CREATE INDEX IF NOT EXISTS idx_commission_records_employee
    ON commission_records (employee_id, created_at);

CREATE INDEX IF NOT EXISTS idx_commission_records_order
    ON commission_records (order_id);
