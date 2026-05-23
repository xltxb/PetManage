-- 056_open_platform_orders: Add pending/paid statuses, order_no, and callback support for open platform orders

-- Expand order status to support open platform flow (pending → paid/completed)
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (status IN ('pending', 'paid', 'completed', 'refunded', 'partially_refunded'));

-- Add external order number for open platform reference
ALTER TABLE orders ADD COLUMN IF NOT EXISTS order_no VARCHAR(32);
CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_order_no ON orders(order_no) WHERE order_no IS NOT NULL;

-- Make product_id nullable to support service items (which have no product)
ALTER TABLE order_items ALTER COLUMN product_id DROP NOT NULL;

-- Create refunds table if not already exists
CREATE TABLE IF NOT EXISTS refunds (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    refund_type VARCHAR(16) NOT NULL CHECK (refund_type IN ('full', 'partial')),
    reason TEXT NOT NULL DEFAULT '',
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'pending_approval', 'approved', 'rejected')),
    requested_by BIGINT NOT NULL,
    approved_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refunds_order_id ON refunds(order_id);
CREATE INDEX IF NOT EXISTS idx_refunds_merchant_id ON refunds(merchant_id);

CREATE TABLE IF NOT EXISTS refund_items (
    id BIGSERIAL PRIMARY KEY,
    refund_id BIGINT NOT NULL REFERENCES refunds(id),
    order_item_id BIGINT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    amount_cents INTEGER NOT NULL CHECK (amount_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refund_items_refund_id ON refund_items(refund_id);
