CREATE TABLE IF NOT EXISTS complaint_tickets (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    complaint_type VARCHAR(50) NOT NULL CHECK (complaint_type IN ('service', 'product', 'staff', 'pricing', 'other')),
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'resolved', 'revisited')),
    assigned_to BIGINT REFERENCES platform_users(id),
    resolution TEXT NOT NULL DEFAULT '',
    revisit_notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_complaint_tickets_merchant ON complaint_tickets(merchant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_complaint_tickets_status ON complaint_tickets(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_complaint_tickets_assigned_to ON complaint_tickets(assigned_to) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_complaint_tickets_created_at ON complaint_tickets(created_at DESC) WHERE deleted_at IS NULL;
