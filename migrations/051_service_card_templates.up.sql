-- 051_service_card_templates: Service card product templates for F064

CREATE TABLE IF NOT EXISTS service_card_templates (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(128) NOT NULL,
    service_item_id BIGINT REFERENCES service_items(id),
    total_uses INTEGER NOT NULL CHECK (total_uses > 0),
    price_cents BIGINT NOT NULL CHECK (price_cents > 0),
    validity_days INTEGER NOT NULL DEFAULT 365,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sct_merchant ON service_card_templates(merchant_id);
CREATE INDEX IF NOT EXISTS idx_sct_status ON service_card_templates(status);
