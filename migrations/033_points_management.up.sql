-- 033: Points Management
-- points_rules: configurable points earning rules
CREATE TABLE IF NOT EXISTS points_rules (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(100) NOT NULL,
    rule_type VARCHAR(20) NOT NULL CHECK (rule_type IN ('consume', 'signin', 'recharge', 'referral')),
    earn_type VARCHAR(10) NOT NULL DEFAULT 'percent' CHECK (earn_type IN ('fixed', 'percent')),
    earn_value INT NOT NULL DEFAULT 0,
    points_to_cent_rate INT NOT NULL DEFAULT 100,
    max_deduct_ratio INT NOT NULL DEFAULT 50,
    expiry_days INT NOT NULL DEFAULT 365,
    status VARCHAR(10) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_points_rules_merchant ON points_rules(merchant_id, deleted_at);

-- point_transactions: audit log for all point changes
CREATE TABLE IF NOT EXISTS point_transactions (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    member_id BIGINT NOT NULL REFERENCES members(id),
    type VARCHAR(20) NOT NULL CHECK (type IN ('earn', 'deduct', 'expire', 'adjust')),
    points INT NOT NULL,
    points_before INT NOT NULL DEFAULT 0,
    points_after INT NOT NULL DEFAULT 0,
    reference_type VARCHAR(30) NOT NULL DEFAULT '',
    reference_id BIGINT,
    rule_id BIGINT,
    operator_id BIGINT,
    notes TEXT NOT NULL DEFAULT '',
    expire_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_point_tx_member ON point_transactions(member_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_point_tx_merchant ON point_transactions(merchant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_point_tx_type ON point_transactions(merchant_id, type);

-- Add points_expire_at to members for expiry tracking
ALTER TABLE members ADD COLUMN IF NOT EXISTS points_expire_at TIMESTAMPTZ;
