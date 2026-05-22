CREATE TABLE IF NOT EXISTS platform_users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(100),
    phone VARCHAR(20),
    email VARCHAR(255),
    role_id BIGINT REFERENCES platform_roles(id),
    merchant_id BIGINT REFERENCES merchants(id),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_platform_users_role_id ON platform_users(role_id);
CREATE INDEX IF NOT EXISTS idx_platform_users_merchant_id ON platform_users(merchant_id);
CREATE INDEX IF NOT EXISTS idx_platform_users_status ON platform_users(status);
CREATE INDEX IF NOT EXISTS idx_platform_users_deleted_at ON platform_users(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_platform_users_phone ON platform_users(phone) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_platform_users_username ON platform_users(username) WHERE deleted_at IS NULL;
