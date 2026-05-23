-- 031: Member Level System
-- member_level_rules: configurable tier rules per merchant
CREATE TABLE IF NOT EXISTS member_level_rules (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(50) NOT NULL,
    level_order INT NOT NULL DEFAULT 0,
    upgrade_type VARCHAR(30) NOT NULL CHECK (upgrade_type IN ('total_spending', 'total_recharge', 'order_count')),
    upgrade_value BIGINT NOT NULL DEFAULT 0,
    discount_percent SMALLINT NOT NULL DEFAULT 100,
    points_multiplier SMALLINT NOT NULL DEFAULT 100,
    downgrade_days INT NOT NULL DEFAULT 180,
    icon VARCHAR(50) NOT NULL DEFAULT '',
    color VARCHAR(20) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(10) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_member_level_rules_merchant ON member_level_rules(merchant_id, deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_member_level_rules_default ON member_level_rules(merchant_id, is_default) WHERE is_default = true AND deleted_at IS NULL;

-- member_level_logs: track level changes (upgrade/downgrade/set_default)
CREATE TABLE IF NOT EXISTS member_level_logs (
    id BIGSERIAL PRIMARY KEY,
    member_id BIGINT NOT NULL REFERENCES members(id),
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    old_level_id BIGINT,
    new_level_id BIGINT NOT NULL REFERENCES member_level_rules(id),
    change_type VARCHAR(20) NOT NULL CHECK (change_type IN ('upgrade', 'downgrade', 'set_default')),
    change_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_member_level_logs_member ON member_level_logs(member_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_member_level_logs_merchant ON member_level_logs(merchant_id);

-- Add level_id to members table
ALTER TABLE members ADD COLUMN IF NOT EXISTS level_id BIGINT REFERENCES member_level_rules(id);
CREATE INDEX IF NOT EXISTS idx_members_level ON members(level_id);
