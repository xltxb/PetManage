-- 046: Accounts payable management
-- payable_records: accounts payable generated from purchase order receipts
-- payment_records: payment transactions against payable records

CREATE TABLE IF NOT EXISTS payable_records (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    supplier_id BIGINT NOT NULL REFERENCES suppliers(id),
    purchase_order_id BIGINT NOT NULL REFERENCES purchase_orders(id),
    total_cents INTEGER NOT NULL DEFAULT 0,
    paid_cents INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'unpaid'
        CHECK (status IN ('unpaid', 'partial', 'paid')),
    due_date DATE,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_payable_records_merchant
    ON payable_records (merchant_id, created_at);

CREATE INDEX IF NOT EXISTS idx_payable_records_supplier
    ON payable_records (supplier_id);

CREATE INDEX IF NOT EXISTS idx_payable_records_po
    ON payable_records (purchase_order_id);

CREATE TABLE IF NOT EXISTS payment_records (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    supplier_id BIGINT NOT NULL REFERENCES suppliers(id),
    payable_record_id BIGINT NOT NULL REFERENCES payable_records(id),
    amount_cents INTEGER NOT NULL,
    payment_method VARCHAR(50) NOT NULL DEFAULT 'bank_transfer'
        CHECK (payment_method IN ('cash', 'bank_transfer', 'check', 'other')),
    payment_date DATE NOT NULL DEFAULT CURRENT_DATE,
    reference_no VARCHAR(100) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_by BIGINT REFERENCES platform_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_payment_records_merchant
    ON payment_records (merchant_id, payment_date);

CREATE INDEX IF NOT EXISTS idx_payment_records_payable
    ON payment_records (payable_record_id);
