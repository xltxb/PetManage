-- Remove immutability trigger and ip_address column.
DROP TRIGGER IF EXISTS trg_operation_logs_immutable ON operation_logs;
DROP FUNCTION IF EXISTS prevent_operation_log_mutation();
ALTER TABLE operation_logs DROP COLUMN IF EXISTS ip_address;
DROP INDEX IF EXISTS idx_operation_logs_action;
