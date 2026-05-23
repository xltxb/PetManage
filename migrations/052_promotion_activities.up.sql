-- 052_promotion_activities: Promotion activity management for F065

CREATE TABLE IF NOT EXISTS promotion_activities (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(128) NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('full_reduction', 'discount', 'flash_sale', 'group_buy')),
    rules JSONB NOT NULL DEFAULT '{}',
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    total_order_count INT NOT NULL DEFAULT 0,
    total_discount_cents BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'ended')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pa_merchant ON promotion_activities(merchant_id);
CREATE INDEX IF NOT EXISTS idx_pa_type ON promotion_activities(type);
CREATE INDEX IF NOT EXISTS idx_pa_status ON promotion_activities(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pa_time ON promotion_activities(start_time, end_time) WHERE deleted_at IS NULL;

-- Add promotion tracking to orders
ALTER TABLE orders ADD COLUMN IF NOT EXISTS promotion_activity_id BIGINT REFERENCES promotion_activities(id);
