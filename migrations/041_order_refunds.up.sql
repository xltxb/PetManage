-- 041_order_refunds: Refund support tables and order status expansion

-- Expand order status to support partial refunds. Need to drop and recreate the check constraint.
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (status IN ('completed', 'refunded', 'partially_refunded'));

-- Refund records table.
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

-- Individual items within a refund (for partial refund tracking).
CREATE TABLE IF NOT EXISTS refund_items (
    id BIGSERIAL PRIMARY KEY,
    refund_id BIGINT NOT NULL REFERENCES refunds(id),
    order_item_id BIGINT NOT NULL REFERENCES order_items(id),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    amount_cents INTEGER NOT NULL CHECK (amount_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refund_items_refund_id ON refund_items(refund_id);
