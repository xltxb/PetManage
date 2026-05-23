-- 041_order_refunds down migration

DROP TABLE IF EXISTS refund_items;
DROP TABLE IF EXISTS refunds;

-- Revert orders status check to original.
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (status IN ('completed', 'refunded'));
