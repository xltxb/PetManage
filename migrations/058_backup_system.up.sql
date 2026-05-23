CREATE TABLE IF NOT EXISTS backup_records (
    id BIGSERIAL PRIMARY KEY,
    backup_type VARCHAR(20) NOT NULL CHECK (backup_type IN ('full', 'incremental')),
    file_path TEXT NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_backup_records_type ON backup_records(backup_type);
CREATE INDEX IF NOT EXISTS idx_backup_records_status ON backup_records(status);
CREATE INDEX IF NOT EXISTS idx_backup_records_started_at ON backup_records(started_at);
