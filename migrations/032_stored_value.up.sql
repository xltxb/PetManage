-- 032: Stored Value Management
-- recharge_packages: predefined top-up packages with bonus
CREATE TABLE IF NOT EXISTS recharge_packages (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(100) NOT NULL,
    principal_cents BIGINT NOT NULL DEFAULT 0,
    bonus_cents BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(10) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_recharge_packages_merchant ON recharge_packages(merchant_id, deleted_at);

-- balance_transactions: audit log for all balance changes
CREATE TABLE IF NOT EXISTS balance_transactions (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    member_id BIGINT NOT NULL REFERENCES members(id),
    type VARCHAR(20) NOT NULL CHECK (type IN ('recharge', 'payment', 'refund', 'adjustment')),
    amount_cents BIGINT NOT NULL,
    principal_before BIGINT NOT NULL DEFAULT 0,
    principal_after BIGINT NOT NULL DEFAULT 0,
    bonus_before BIGINT NOT NULL DEFAULT 0,
    bonus_after BIGINT NOT NULL DEFAULT 0,
    reference_type VARCHAR(30) NOT NULL DEFAULT '',
    reference_id BIGINT,
    operator_id BIGINT,
    payment_method VARCHAR(20) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_balance_tx_member ON balance_transactions(member_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_balance_tx_merchant ON balance_transactions(merchant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_balance_tx_type ON balance_transactions(merchant_id, type);

-- Split balance_cents into principal (paid money) and bonus (gifted money)
ALTER TABLE members ADD COLUMN IF NOT EXISTS principal_balance_cents BIGINT NOT NULL DEFAULT 0;
ALTER TABLE members ADD COLUMN IF NOT EXISTS bonus_balance_cents BIGINT NOT NULL DEFAULT 0;

-- Migrate existing balance to principal_balance (all existing balance treated as principal)
UPDATE members SET principal_balance_cents = balance_cents WHERE principal_balance_cents = 0 AND balance_cents > 0;
