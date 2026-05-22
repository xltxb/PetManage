-- Add review columns to merchants.
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS review_remark TEXT;
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS reviewed_by BIGINT;
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

-- Create operation_logs table for audit trail.
CREATE TABLE IF NOT EXISTS operation_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    action VARCHAR(50) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_id BIGINT NOT NULL,
    detail JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_operation_logs_user_id ON operation_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_operation_logs_target ON operation_logs(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_operation_logs_created_at ON operation_logs(created_at);

-- Seed merchant_admin role for auto-created merchant administrators.
INSERT INTO platform_roles (code, name, permissions) VALUES ('merchant_admin', 'Merchant Admin', '["merchant:*"]') ON CONFLICT (code) DO NOTHING;
