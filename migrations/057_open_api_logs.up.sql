CREATE TABLE IF NOT EXISTS open_api_logs (
    id BIGSERIAL PRIMARY KEY,
    developer_id BIGINT,
    endpoint VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    status_code INT NOT NULL,
    duration_ms INT NOT NULL,
    ip_address VARCHAR(45) NOT NULL DEFAULT '',
    request_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_open_api_logs_developer ON open_api_logs(developer_id, created_at);
CREATE INDEX IF NOT EXISTS idx_open_api_logs_endpoint ON open_api_logs(endpoint, created_at);
CREATE INDEX IF NOT EXISTS idx_open_api_logs_created ON open_api_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_open_api_logs_status ON open_api_logs(status_code);
