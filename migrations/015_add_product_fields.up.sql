-- 015_add_product_fields: Add brand, specification, alert_stock, expiry_date to products

ALTER TABLE products ADD COLUMN IF NOT EXISTS brand VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN IF NOT EXISTS specification VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN IF NOT EXISTS alert_stock INTEGER NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS expiry_date DATE;
