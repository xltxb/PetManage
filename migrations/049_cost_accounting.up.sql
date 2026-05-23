-- Migration 049: Cost accounting
-- Add cost_cents to order_items for frozen cost tracking at checkout
ALTER TABLE IF EXISTS order_items ADD COLUMN IF NOT EXISTS cost_cents INTEGER NOT NULL DEFAULT 0;

-- Add cost_cents to service_items for consumables cost tracking
ALTER TABLE IF EXISTS service_items ADD COLUMN IF NOT EXISTS cost_cents INTEGER NOT NULL DEFAULT 0;

-- Fixed expenses table (rent, utilities, salary, etc.)
CREATE TABLE IF NOT EXISTS fixed_expenses (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    name VARCHAR(200) NOT NULL,
    amount_cents INTEGER NOT NULL DEFAULT 0,
    category VARCHAR(50) NOT NULL DEFAULT 'other',
    notes TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_fixed_expenses_merchant_id ON fixed_expenses(merchant_id);
CREATE INDEX IF NOT EXISTS idx_fixed_expenses_category ON fixed_expenses(merchant_id, category);
