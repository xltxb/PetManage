ALTER TABLE notifications ADD COLUMN IF NOT EXISTS send_status VARCHAR(20) NOT NULL DEFAULT 'success';
CREATE INDEX IF NOT EXISTS idx_notifications_send_status ON notifications(send_status);
CREATE INDEX IF NOT EXISTS idx_notifications_category ON notifications(category);
