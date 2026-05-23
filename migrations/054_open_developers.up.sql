-- Open platform developer onboarding
CREATE TABLE IF NOT EXISTS open_developers (
    id BIGSERIAL PRIMARY KEY,
    company_name VARCHAR(255) NOT NULL,
    contact_person VARCHAR(100) NOT NULL,
    contact_phone VARCHAR(50) NOT NULL,
    contact_email VARCHAR(255) NOT NULL,
    usage_purpose TEXT NOT NULL DEFAULT '',
    callback_url VARCHAR(500) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    app_key VARCHAR(64),
    app_secret VARCHAR(128),
    permissions JSONB NOT NULL DEFAULT '[]',
    review_remark TEXT NOT NULL DEFAULT '',
    reviewed_by BIGINT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_open_developers_app_key ON open_developers(app_key) WHERE app_key IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_open_developers_status ON open_developers(status, deleted_at);
