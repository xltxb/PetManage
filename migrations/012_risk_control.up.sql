-- Risk control tables: risk rules and alerts

CREATE TABLE IF NOT EXISTS risk_rules (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    rule_type VARCHAR(50) NOT NULL CHECK (rule_type IN ('large_refund', 'high_frequency')),
    threshold_cents INTEGER NOT NULL DEFAULT 0,
    threshold_count INTEGER NOT NULL DEFAULT 0,
    time_window_minutes INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS risk_alerts (
    id BIGSERIAL PRIMARY KEY,
    rule_id BIGINT NOT NULL REFERENCES risk_rules(id),
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    order_id BIGINT REFERENCES orders(id),
    member_id BIGINT,
    alert_type VARCHAR(50) NOT NULL CHECK (alert_type IN ('large_refund', 'high_frequency')),
    description TEXT NOT NULL DEFAULT '',
    detail JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processed', 'ignored')),
    handled_by BIGINT REFERENCES platform_users(id),
    handled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_risk_alerts_merchant_id ON risk_alerts(merchant_id);
CREATE INDEX IF NOT EXISTS idx_risk_alerts_status ON risk_alerts(status);
CREATE INDEX IF NOT EXISTS idx_risk_alerts_alert_type ON risk_alerts(alert_type);
CREATE INDEX IF NOT EXISTS idx_risk_alerts_created_at ON risk_alerts(created_at);

-- Seed default risk rules
INSERT INTO risk_rules (name, rule_type, threshold_cents, threshold_count, time_window_minutes, enabled)
VALUES
    ('单笔退款金额大于5000元', 'large_refund', 500000, 0, 0, true),
    ('同一用户1小时内交易超过10笔', 'high_frequency', 0, 10, 60, true)
ON CONFLICT DO NOTHING;
