-- 047: Notification settings, templates, and channel support.

-- Notification scenarios settings per merchant (enable/disable scenarios and channels).
CREATE TABLE IF NOT EXISTS notification_settings (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    scenario VARCHAR(40) NOT NULL,
    sms_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    wechat_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    system_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ns_merchant_scenario
    ON notification_settings(merchant_id, scenario);

-- Notification templates (SMS signature + template ID, WeChat template ID).
CREATE TABLE IF NOT EXISTS notification_templates (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    scenario VARCHAR(40) NOT NULL,
    channel VARCHAR(10) NOT NULL CHECK (channel IN ('sms', 'wechat')),
    template_id VARCHAR(100) NOT NULL DEFAULT '',
    signature VARCHAR(30) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_nt_merchant_scenario_channel
    ON notification_templates(merchant_id, scenario, channel);

-- Add channel column to notifications table.
ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS channel VARCHAR(10) NOT NULL DEFAULT 'system';
