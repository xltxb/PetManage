ALTER TABLE stock_flows DROP COLUMN IF EXISTS product_sku_id;
ALTER TABLE order_items DROP COLUMN IF EXISTS sku_spec_info;
ALTER TABLE order_items DROP COLUMN IF EXISTS product_sku_id;
DROP TABLE IF EXISTS product_skus;
