-- Pet service records: stores completed service history for each pet
CREATE TABLE IF NOT EXISTS pet_service_records (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    member_id BIGINT NOT NULL,
    pet_id BIGINT NOT NULL REFERENCES pets(id),
    appointment_id BIGINT REFERENCES appointments(id),
    service_item_id BIGINT NOT NULL,
    employee_id BIGINT NOT NULL,
    service_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    materials_used JSONB NOT NULL DEFAULT '{}',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pet_service_records_pet ON pet_service_records(pet_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_pet_service_records_member ON pet_service_records(member_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_pet_service_records_merchant ON pet_service_records(merchant_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_pet_service_records_appointment ON pet_service_records(appointment_id);

-- Service evaluations: customer ratings and feedback after service
CREATE TABLE IF NOT EXISTS service_evaluations (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    member_id BIGINT NOT NULL,
    pet_id BIGINT REFERENCES pets(id),
    appointment_id BIGINT REFERENCES appointments(id),
    service_record_id BIGINT NOT NULL REFERENCES pet_service_records(id),
    employee_id BIGINT NOT NULL,
    rating INT NOT NULL DEFAULT 5,
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_eval_rating CHECK (rating >= 1 AND rating <= 5)
);

CREATE INDEX IF NOT EXISTS idx_service_evaluations_service ON service_evaluations(service_record_id);
CREATE INDEX IF NOT EXISTS idx_service_evaluations_employee ON service_evaluations(employee_id);
CREATE INDEX IF NOT EXISTS idx_service_evaluations_pet ON service_evaluations(pet_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_service_evaluations_unique ON service_evaluations(service_record_id);
