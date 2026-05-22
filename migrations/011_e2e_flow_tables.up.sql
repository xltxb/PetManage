-- 011_e2e_flow_tables: Core business flow tables for E2E testing
-- products, orders, order_items, payments, stock_flows

CREATE TABLE IF NOT EXISTS products (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    barcode VARCHAR(100) NOT NULL DEFAULT '',
    name VARCHAR(255) NOT NULL,
    price_cents INTEGER NOT NULL,
    cost_cents INTEGER NOT NULL DEFAULT 0,
    stock INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_products_merchant_id ON products(merchant_id);
CREATE INDEX IF NOT EXISTS idx_products_barcode ON products(merchant_id, barcode);
CREATE INDEX IF NOT EXISTS idx_products_status ON products(merchant_id, status);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    member_id BIGINT,
    total_cents INTEGER NOT NULL,
    paid_cents INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'refunded')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_merchant_id ON orders(merchant_id);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(merchant_id, created_at);

CREATE TABLE IF NOT EXISTS order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    product_id BIGINT NOT NULL REFERENCES products(id),
    product_name VARCHAR(255) NOT NULL,
    price_cents INTEGER NOT NULL,
    quantity INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);

CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    method VARCHAR(50) NOT NULL CHECK (method IN ('cash', 'wechat', 'alipay', 'balance')),
    amount_cents INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_order_id ON payments(order_id);

CREATE TABLE IF NOT EXISTS stock_flows (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    product_id BIGINT NOT NULL REFERENCES products(id),
    order_id BIGINT,
    type VARCHAR(50) NOT NULL CHECK (type IN ('sale', 'inbound', 'adjustment')),
    quantity_change INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_stock_flows_merchant_id ON stock_flows(merchant_id);
CREATE INDEX IF NOT EXISTS idx_stock_flows_product_id ON stock_flows(product_id);
CREATE INDEX IF NOT EXISTS idx_stock_flows_order_id ON stock_flows(order_id);
