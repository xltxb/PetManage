-- 056_open_platform_orders: Revert open platform orders changes

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (status IN ('completed', 'refunded', 'partially_refunded'));

ALTER TABLE orders DROP COLUMN IF EXISTS order_no;

DROP INDEX IF EXISTS idx_refund_items_refund_id;
DROP TABLE IF EXISTS refund_items;

DROP INDEX IF EXISTS idx_refunds_merchant_id;
DROP INDEX IF EXISTS idx_refunds_order_id;
DROP TABLE IF EXISTS refunds;
