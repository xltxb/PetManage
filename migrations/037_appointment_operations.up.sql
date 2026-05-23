-- Appointment change log records
CREATE TABLE IF NOT EXISTS appointment_change_logs (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    appointment_id BIGINT NOT NULL REFERENCES appointments(id),
    action VARCHAR(20) NOT NULL,
    old_value JSONB,
    new_value JSONB,
    operator_id BIGINT,
    reason TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_appointment_change_logs_appt ON appointment_change_logs(appointment_id);

-- Simple notification records
CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    user_type VARCHAR(20) NOT NULL DEFAULT 'member',
    title VARCHAR(200) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    category VARCHAR(30) NOT NULL DEFAULT 'appointment',
    related_id BIGINT,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(merchant_id, user_id, user_type);
CREATE INDEX IF NOT EXISTS idx_notifications_created ON notifications(created_at);
