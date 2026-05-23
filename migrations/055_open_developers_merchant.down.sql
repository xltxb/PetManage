-- Remove merchant_id from open_developers
ALTER TABLE IF EXISTS open_developers DROP COLUMN IF EXISTS merchant_id;
