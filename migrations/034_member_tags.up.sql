-- 034: Member Tags Management
-- member_tags: tag definitions per merchant
CREATE TABLE IF NOT EXISTS member_tags (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    name VARCHAR(50) NOT NULL,
    color VARCHAR(20) NOT NULL DEFAULT '#3B82F6',
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(10) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_member_tags_merchant ON member_tags(merchant_id, deleted_at);

-- member_tag_relations: many-to-many between members and tags
CREATE TABLE IF NOT EXISTS member_tag_relations (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    member_id BIGINT NOT NULL REFERENCES members(id),
    tag_id BIGINT NOT NULL REFERENCES member_tags(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(member_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_member_tag_relations_member ON member_tag_relations(member_id);
CREATE INDEX IF NOT EXISTS idx_member_tag_relations_tag ON member_tag_relations(tag_id);
CREATE INDEX IF NOT EXISTS idx_member_tag_relations_merchant ON member_tag_relations(merchant_id);

-- member_tag_rules: auto-tagging rules
CREATE TABLE IF NOT EXISTS member_tag_rules (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    tag_id BIGINT NOT NULL REFERENCES member_tags(id),
    rule_type VARCHAR(30) NOT NULL CHECK (rule_type IN ('total_consumption', 'order_count', 'pet_count', 'last_visit_days')),
    operator VARCHAR(5) NOT NULL DEFAULT 'gte' CHECK (operator IN ('gte', 'lte', 'eq')),
    threshold_value DECIMAL(16,2) NOT NULL DEFAULT 0,
    status VARCHAR(10) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_member_tag_rules_merchant ON member_tag_rules(merchant_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_member_tag_rules_tag ON member_tag_rules(tag_id);
