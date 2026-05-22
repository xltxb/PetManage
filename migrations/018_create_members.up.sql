-- 018: members — member card opening and archive management
CREATE TABLE IF NOT EXISTS members (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    card_no VARCHAR(30) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    wechat VARCHAR(50) NOT NULL DEFAULT '',
    gender VARCHAR(1) NOT NULL DEFAULT '',
    birthday DATE,
    address VARCHAR(500) NOT NULL DEFAULT '',
    remark TEXT NOT NULL DEFAULT '',
    balance_cents BIGINT NOT NULL DEFAULT 0,
    points INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_member_status CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX idx_members_merchant_id ON members(merchant_id);
CREATE INDEX idx_members_card_no ON members(card_no);
CREATE INDEX idx_members_phone ON members(phone);
CREATE INDEX idx_members_name ON members(merchant_id, name);
