-- 044: Employee attendance management
-- attendance_records: check-in/check-out daily records
-- leave_requests: leave/absence applications
-- overtime_records: overtime registrations

CREATE TABLE IF NOT EXISTS attendance_records (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    record_date DATE NOT NULL,
    check_in_time TIMESTAMPTZ,
    check_out_time TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'normal'
        CHECK (status IN ('normal', 'late', 'early_leave', 'absent')),
    late_minutes INT NOT NULL DEFAULT 0,
    early_leave_minutes INT NOT NULL DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_attendance_record_unique
    ON attendance_records (merchant_id, employee_id, record_date)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_attendance_records_merchant
    ON attendance_records (merchant_id, record_date);

CREATE TABLE IF NOT EXISTS leave_requests (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    leave_type VARCHAR(20) NOT NULL DEFAULT 'personal'
        CHECK (leave_type IN ('annual', 'sick', 'personal', 'other')),
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    reason TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    reviewed_by BIGINT REFERENCES employees(id),
    review_remark TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_leave_requests_merchant
    ON leave_requests (merchant_id, status);

CREATE INDEX IF NOT EXISTS idx_leave_requests_employee
    ON leave_requests (employee_id, created_at);

CREATE TABLE IF NOT EXISTS overtime_records (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    overtime_date DATE NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    duration_hours NUMERIC(4,1) NOT NULL DEFAULT 0,
    reason TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by BIGINT REFERENCES employees(id),
    review_remark TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_overtime_records_merchant
    ON overtime_records (merchant_id, status);

CREATE INDEX IF NOT EXISTS idx_overtime_records_employee
    ON overtime_records (employee_id, created_at);
