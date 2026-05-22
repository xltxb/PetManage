CREATE TABLE IF NOT EXISTS merchant_roles (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(100) NOT NULL,
    code VARCHAR(50) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    permissions JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_merchant_roles_code ON merchant_roles (merchant_id, code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchant_roles_merchant ON merchant_roles (merchant_id);

ALTER TABLE employees ADD COLUMN IF NOT EXISTS merchant_role_id BIGINT REFERENCES merchant_roles(id);
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS employee_id BIGINT REFERENCES employees(id);
