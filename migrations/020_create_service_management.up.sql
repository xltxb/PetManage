-- Service categories for merchants
CREATE TABLE IF NOT EXISTS service_categories (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    name VARCHAR(100) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_service_categories_merchant ON service_categories(merchant_id) WHERE deleted_at IS NULL;

-- Service items within categories
CREATE TABLE IF NOT EXISTS service_items (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    category_id BIGINT NOT NULL,
    name VARCHAR(200) NOT NULL,
    duration_minutes INT NOT NULL DEFAULT 30,
    price_cents INT NOT NULL DEFAULT 0,
    member_price_cents INT NOT NULL DEFAULT 0,
    pet_type VARCHAR(50) NOT NULL DEFAULT '',
    min_weight_kg NUMERIC(8,2) NOT NULL DEFAULT 0,
    max_weight_kg NUMERIC(8,2) NOT NULL DEFAULT 0,
    materials TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_service_items_merchant ON service_items(merchant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_service_items_category ON service_items(category_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_service_items_status ON service_items(status) WHERE deleted_at IS NULL;
