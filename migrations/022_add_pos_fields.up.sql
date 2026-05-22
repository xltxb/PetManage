-- Add notes column to orders for POS order notes
ALTER TABLE orders ADD COLUMN IF NOT EXISTS notes TEXT;

-- Add service_item_id to order_items for service item purchases
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS service_item_id BIGINT;

-- Make product_id nullable to support service items (which have no product)
ALTER TABLE order_items ALTER COLUMN product_id DROP NOT NULL;
