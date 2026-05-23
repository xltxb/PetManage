CREATE TABLE IF NOT EXISTS appointments (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    member_id BIGINT NOT NULL,
    pet_id BIGINT NOT NULL,
    service_item_id BIGINT NOT NULL,
    employee_id BIGINT NOT NULL,
    appointment_time TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    remark TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_appointments_merchant ON appointments(merchant_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_appointments_employee_time ON appointments(employee_id, appointment_time);
CREATE INDEX IF NOT EXISTS idx_appointments_time ON appointments(appointment_time);
CREATE INDEX IF NOT EXISTS idx_appointments_member ON appointments(member_id);
