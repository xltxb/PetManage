CREATE TABLE IF NOT EXISTS coupons (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    code VARCHAR(64) NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('fixed', 'percent')),
    value_cents INT NOT NULL CHECK (value_cents > 0),
    min_order_cents INT NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'used', 'expired', 'disabled')),
    used_at TIMESTAMPTZ,
    used_by_member_id BIGINT,
    used_order_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_coupons_code ON coupons(code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_coupons_merchant ON coupons(merchant_id);
