CREATE TABLE IF NOT EXISTS platform_announcements (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    scope VARCHAR(20) NOT NULL DEFAULT 'all' CHECK (scope IN ('all', 'merchants')),
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    publish_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by BIGINT NOT NULL REFERENCES platform_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_announcements_pinned_publish ON platform_announcements(is_pinned DESC, publish_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_announcements_scope ON platform_announcements(scope) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS announcement_merchants (
    id BIGSERIAL PRIMARY KEY,
    announcement_id BIGINT NOT NULL REFERENCES platform_announcements(id) ON DELETE CASCADE,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    UNIQUE(announcement_id, merchant_id)
);

CREATE INDEX IF NOT EXISTS idx_announcement_merchants_merchant ON announcement_merchants(merchant_id);

CREATE TABLE IF NOT EXISTS announcement_reads (
    id BIGSERIAL PRIMARY KEY,
    announcement_id BIGINT NOT NULL REFERENCES platform_announcements(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(announcement_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_announcement_reads_user ON announcement_reads(user_id);
