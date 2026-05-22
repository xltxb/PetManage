-- Add ip_address column to operation_logs.
ALTER TABLE operation_logs ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45);
COMMENT ON COLUMN operation_logs.ip_address IS 'Client IP address at the time of the operation';

-- Add index on action column for filtered queries.
CREATE INDEX IF NOT EXISTS idx_operation_logs_action ON operation_logs(action);

-- Immutability trigger: prevent UPDATE and DELETE on operation_logs.
CREATE OR REPLACE FUNCTION prevent_operation_log_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'operation_logs table is append-only; UPDATE and DELETE are not allowed';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_operation_logs_immutable ON operation_logs;
CREATE TRIGGER trg_operation_logs_immutable
    BEFORE UPDATE OR DELETE ON operation_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_operation_log_mutation();
