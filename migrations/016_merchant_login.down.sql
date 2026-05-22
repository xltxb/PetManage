-- 016_merchant_login: Remove login failure tracking columns
ALTER TABLE platform_users
    DROP COLUMN IF EXISTS login_fail_count,
    DROP COLUMN IF EXISTS locked_until;
