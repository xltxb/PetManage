-- 019: pets — member pet archive management
CREATE TABLE IF NOT EXISTS pets (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    member_id BIGINT NOT NULL REFERENCES members(id),
    name VARCHAR(100) NOT NULL,
    breed VARCHAR(100) NOT NULL DEFAULT '',
    gender VARCHAR(1) NOT NULL DEFAULT '',
    age INT NOT NULL DEFAULT 0,
    weight VARCHAR(20) NOT NULL DEFAULT '',
    vaccine_records JSONB NOT NULL DEFAULT '[]',
    deworming_records JSONB NOT NULL DEFAULT '[]',
    allergy_history TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_pet_gender CHECK (gender IN ('', 'M', 'F')),
    CONSTRAINT chk_pet_status CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX idx_pets_member_id ON pets(member_id);
CREATE INDEX idx_pets_merchant_id ON pets(merchant_id);
