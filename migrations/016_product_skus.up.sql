-- 016: product_skus — multi-spec SKU management
CREATE TABLE IF NOT EXISTS product_skus (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id),
    sku_code VARCHAR(100) NOT NULL DEFAULT '',
    spec_info JSONB NOT NULL DEFAULT '{}',
    price_cents INT NOT NULL DEFAULT 0,
    cost_cents INT NOT NULL DEFAULT 0,
    stock INT NOT NULL DEFAULT 0,
    alert_stock INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_product_sku_status CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX idx_product_skus_product_id ON product_skus(product_id);
CREATE UNIQUE INDEX idx_product_skus_code ON product_skus(product_id, sku_code) WHERE sku_code != '' AND deleted_at IS NULL;
CREATE INDEX idx_product_skus_status ON product_skus(status);

-- Add sku_id reference to order_items
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS product_sku_id BIGINT;
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS sku_spec_info JSONB;

-- Add sku_id reference to stock_flows
ALTER TABLE stock_flows ADD COLUMN IF NOT EXISTS product_sku_id BIGINT;
