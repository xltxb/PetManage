-- Rollback 031
ALTER TABLE members DROP COLUMN IF EXISTS level_id;
DROP TABLE IF EXISTS member_level_logs;
DROP TABLE IF EXISTS member_level_rules;
