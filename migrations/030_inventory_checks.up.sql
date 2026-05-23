-- 030: inventory count checks — count check management with auto profit/loss calculation
CREATE TABLE IF NOT EXISTS inventory_checks (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    check_no VARCHAR(30) NOT NULL,
    check_type VARCHAR(20) NOT NULL CHECK (check_type IN ('full', 'category', 'product')),
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'counting', 'review', 'pending_approve', 'completed')),
    scope_data JSONB NOT NULL DEFAULT '{}',
    threshold_percent NUMERIC(5,2) NOT NULL DEFAULT 5.00,
    operator_id BIGINT,
    operator_name VARCHAR(255) NOT NULL DEFAULT '',
    notes VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_inventory_checks_merchant_id ON inventory_checks(merchant_id);
CREATE UNIQUE INDEX idx_inventory_checks_check_no ON inventory_checks(merchant_id, check_no) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS inventory_check_items (
    id BIGSERIAL PRIMARY KEY,
    check_id BIGINT NOT NULL REFERENCES inventory_checks(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id),
    product_sku_id BIGINT,
    product_name VARCHAR(255) NOT NULL DEFAULT '',
    system_stock INT NOT NULL DEFAULT 0,
    actual_stock INT,
    diff_quantity INT,
    cost_cents INT NOT NULL DEFAULT 0,
    diff_amount_cents INT,
    notes VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_inventory_check_items_check_id ON inventory_check_items(check_id);
CREATE INDEX idx_inventory_check_items_product_id ON inventory_check_items(product_id);
