CREATE TABLE IF NOT EXISTS employee_schedules (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    employee_id BIGINT NOT NULL,
    schedule_date DATE NOT NULL,
    shift_type VARCHAR(20) NOT NULL DEFAULT 'morning' CHECK (shift_type IN ('morning', 'evening', 'rest')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_es_merchant ON employee_schedules(merchant_id);
CREATE INDEX idx_es_employee ON employee_schedules(employee_id);
CREATE INDEX idx_es_date ON employee_schedules(schedule_date);
CREATE UNIQUE INDEX idx_es_employee_date ON employee_schedules(employee_id, schedule_date) WHERE deleted_at IS NULL;
