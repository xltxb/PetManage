-- 043_receipt_templates: Per-merchant receipt template customization
CREATE TABLE IF NOT EXISTS receipt_templates (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL UNIQUE REFERENCES merchants(id),
    logo_url TEXT NOT NULL DEFAULT '',
    store_name VARCHAR(128) NOT NULL DEFAULT '',
    contact_phone VARCHAR(32) NOT NULL DEFAULT '',
    contact_address VARCHAR(256) NOT NULL DEFAULT '',
    footer_note TEXT NOT NULL DEFAULT '',
    paper_width VARCHAR(8) NOT NULL DEFAULT '80mm' CHECK (paper_width IN ('58mm', '80mm')),
    show_qrcode BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_receipt_templates_merchant ON receipt_templates(merchant_id);
