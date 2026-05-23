DROP INDEX IF EXISTS idx_notifications_category;
DROP INDEX IF EXISTS idx_notifications_send_status;
ALTER TABLE notifications DROP COLUMN IF EXISTS send_status;
