-- Add merchant association to open platform developers
ALTER TABLE IF EXISTS open_developers ADD COLUMN IF NOT EXISTS merchant_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_open_developers_merchant ON open_developers(merchant_id) WHERE merchant_id IS NOT NULL AND deleted_at IS NULL;
