-- Coupon template management for F063: coupon rules & issuance
CREATE TABLE IF NOT EXISTS coupon_templates (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    description TEXT DEFAULT '',
    type VARCHAR(32) NOT NULL CHECK (type IN ('full_reduction', 'discount', 'cash_voucher', 'new_member', 'birthday')),
    value_cents INT NOT NULL CHECK (value_cents > 0),
    min_order_cents INT NOT NULL DEFAULT 0,
    max_discount_cents INT DEFAULT 0,
    validity_days INT NOT NULL DEFAULT 30,
    max_claims_per_member INT NOT NULL DEFAULT 1,
    applicable_categories TEXT DEFAULT '',
    issue_method VARCHAR(32) NOT NULL DEFAULT 'manual' CHECK (issue_method IN ('manual', 'auto_new_member', 'auto_birthday')),
    total_issued INT NOT NULL DEFAULT 0,
    total_used INT NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_coupon_templates_merchant ON coupon_templates(merchant_id);
CREATE INDEX IF NOT EXISTS idx_coupon_templates_status ON coupon_templates(status) WHERE deleted_at IS NULL;

-- Individual coupon code instances issued from templates
CREATE TABLE IF NOT EXISTS coupon_codes (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES coupon_templates(id),
    merchant_id BIGINT NOT NULL,
    member_id BIGINT,
    code VARCHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'used', 'expired', 'disabled')),
    used_at TIMESTAMPTZ,
    used_order_id BIGINT,
    expires_at TIMESTAMPTZ NOT NULL,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_coupon_codes_code ON coupon_codes(code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_coupon_codes_template ON coupon_codes(template_id);
CREATE INDEX IF NOT EXISTS idx_coupon_codes_member ON coupon_codes(member_id);
CREATE INDEX IF NOT EXISTS idx_coupon_codes_merchant ON coupon_codes(merchant_id);
