CREATE TABLE IF NOT EXISTS merchant_contracts (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    contract_number VARCHAR(100) NOT NULL,
    file_name VARCHAR(500) NOT NULL,
    file_path VARCHAR(1000) NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    is_current BOOLEAN NOT NULL DEFAULT false,
    prev_contract_id BIGINT,
    uploaded_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_merchant_contracts_merchant_id ON merchant_contracts(merchant_id);
CREATE INDEX IF NOT EXISTS idx_merchant_contracts_status ON merchant_contracts(status);
CREATE INDEX IF NOT EXISTS idx_merchant_contracts_end_date ON merchant_contracts(end_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_merchant_contracts_current ON merchant_contracts(merchant_id, is_current) WHERE is_current = true;
