-- 035: Service Package Management
-- service_packages: combo packages bundling multiple service items
CREATE TABLE IF NOT EXISTS service_packages (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    total_price_cents INT NOT NULL DEFAULT 0,
    original_price_cents INT NOT NULL DEFAULT 0,
    valid_days INT NOT NULL DEFAULT 0,
    usage_limit INT NOT NULL DEFAULT 0,
    status VARCHAR(10) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_service_packages_merchant ON service_packages(merchant_id, deleted_at);

-- service_package_items: individual service items within a package
CREATE TABLE IF NOT EXISTS service_package_items (
    id BIGSERIAL PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES service_packages(id),
    service_item_id BIGINT NOT NULL REFERENCES service_items(id),
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_service_package_items_package ON service_package_items(package_id);
CREATE INDEX IF NOT EXISTS idx_service_package_items_item ON service_package_items(service_item_id);
