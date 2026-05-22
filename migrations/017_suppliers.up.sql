-- 017: suppliers — supplier archive management
CREATE TABLE IF NOT EXISTS suppliers (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(200) NOT NULL,
    contact_person VARCHAR(100) NOT NULL DEFAULT '',
    contact_phone VARCHAR(50) NOT NULL DEFAULT '',
    contact_email VARCHAR(200) NOT NULL DEFAULT '',
    address VARCHAR(500) NOT NULL DEFAULT '',
    settlement_cycle VARCHAR(50) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_supplier_status CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX idx_suppliers_merchant_id ON suppliers(merchant_id);
CREATE INDEX idx_suppliers_status ON suppliers(status);
CREATE INDEX idx_suppliers_name ON suppliers(merchant_id, name);

CREATE TABLE IF NOT EXISTS supplier_products (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES suppliers(id),
    product_id BIGINT NOT NULL REFERENCES products(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(supplier_id, product_id)
);

CREATE INDEX idx_supplier_products_supplier_id ON supplier_products(supplier_id);
CREATE INDEX idx_supplier_products_product_id ON supplier_products(product_id);
