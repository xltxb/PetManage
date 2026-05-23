-- 047 down: Remove notification settings, templates, and channel column.

ALTER TABLE notifications DROP COLUMN IF EXISTS channel;
DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS notification_settings;
