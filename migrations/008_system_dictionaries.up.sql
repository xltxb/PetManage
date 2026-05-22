CREATE TABLE IF NOT EXISTS system_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    parent_id BIGINT,
    level INT NOT NULL DEFAULT 1 CHECK (level IN (1, 2)),
    sort_order INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'enabled' CHECK (status IN ('enabled', 'disabled')),
    is_platform BOOLEAN NOT NULL DEFAULT true,
    merchant_id BIGINT REFERENCES merchants(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_system_categories_parent ON system_categories(parent_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_system_categories_merchant ON system_categories(merchant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_system_categories_platform ON system_categories(is_platform) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS system_breeds (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    pet_type VARCHAR(50) NOT NULL DEFAULT 'dog',
    sort_order INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'enabled' CHECK (status IN ('enabled', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_system_breeds_pet_type ON system_breeds(pet_type) WHERE deleted_at IS NULL;
