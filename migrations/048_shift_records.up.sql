-- 048_shift_records: Daily shift reconciliation for cashier handover

CREATE TABLE IF NOT EXISTS shift_records (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    shift_date DATE NOT NULL,
    expected_total_cents INTEGER NOT NULL DEFAULT 0,
    expected_breakdown JSONB NOT NULL DEFAULT '{}',
    actual_total_cents INTEGER NOT NULL DEFAULT 0,
    actual_breakdown JSONB NOT NULL DEFAULT '{}',
    difference_cents INTEGER NOT NULL DEFAULT 0,
    difference_breakdown JSONB NOT NULL DEFAULT '{}',
    order_count INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed')),
    confirmed_by BIGINT REFERENCES employees(id),
    confirmed_at TIMESTAMPTZ,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_shift_records_merchant_id ON shift_records(merchant_id);
CREATE INDEX IF NOT EXISTS idx_shift_records_employee_id ON shift_records(employee_id);
CREATE INDEX IF NOT EXISTS idx_shift_records_date ON shift_records(merchant_id, shift_date);
CREATE INDEX IF NOT EXISTS idx_shift_records_status ON shift_records(merchant_id, status);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'employees' AND column_name = 'shift_locked'
    ) THEN
        ALTER TABLE employees ADD COLUMN shift_locked BOOLEAN NOT NULL DEFAULT false;
    END IF;
END $$;
