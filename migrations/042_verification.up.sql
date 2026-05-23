-- 042_verification: Service cards, third-party vouchers, and unified verification records

-- Service cards: multi-use cards for service packages (e.g., "5-time bath card")
CREATE TABLE IF NOT EXISTS service_cards (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    member_id BIGINT REFERENCES members(id),
    code VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    service_item_id BIGINT REFERENCES service_items(id),
    total_uses INTEGER NOT NULL CHECK (total_uses > 0),
    used_count INTEGER NOT NULL DEFAULT 0,
    remaining_uses INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'used', 'expired')),
    valid_from TIMESTAMPTZ,
    valid_until TIMESTAMPTZ,
    order_id BIGINT REFERENCES orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_service_cards_code ON service_cards(code) WHERE status != 'expired';
CREATE INDEX IF NOT EXISTS idx_service_cards_merchant ON service_cards(merchant_id);
CREATE INDEX IF NOT EXISTS idx_service_cards_member ON service_cards(member_id);

-- Third-party vouchers: group-buying codes from Meituan, Dianping, etc.
CREATE TABLE IF NOT EXISTS third_party_vouchers (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    code VARCHAR(128) NOT NULL,
    source VARCHAR(64) NOT NULL DEFAULT 'other',
    name VARCHAR(256) NOT NULL,
    service_item_id BIGINT REFERENCES service_items(id),
    amount_cents BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'verified', 'expired')),
    verified_at TIMESTAMPTZ,
    verified_by BIGINT,
    verified_order_id BIGINT REFERENCES orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_third_party_vouchers_code ON third_party_vouchers(code);
CREATE INDEX IF NOT EXISTS idx_third_party_vouchers_merchant ON third_party_vouchers(merchant_id);

-- Unified verification records: log of all verification actions
CREATE TABLE IF NOT EXISTS verification_records (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    verification_type VARCHAR(32) NOT NULL CHECK (verification_type IN ('coupon', 'service_card', 'third_party_voucher')),
    code VARCHAR(128) NOT NULL,
    reference_id BIGINT NOT NULL,
    result VARCHAR(16) NOT NULL CHECK (result IN ('success', 'failed')),
    detail TEXT NOT NULL DEFAULT '',
    order_id BIGINT REFERENCES orders(id),
    verified_by BIGINT NOT NULL,
    verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_verification_records_merchant ON verification_records(merchant_id);
CREATE INDEX IF NOT EXISTS idx_verification_records_type ON verification_records(verification_type);
CREATE INDEX IF NOT EXISTS idx_verification_records_code ON verification_records(code);
CREATE INDEX IF NOT EXISTS idx_verification_records_order ON verification_records(order_id);
