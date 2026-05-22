-- 016_merchant_login: Add login failure tracking columns for account lockout
ALTER TABLE platform_users
    ADD COLUMN login_fail_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN locked_until TIMESTAMPTZ;
