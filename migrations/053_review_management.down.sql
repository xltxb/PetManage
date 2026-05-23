ALTER TABLE service_evaluations
    DROP COLUMN IF EXISTS images,
    DROP COLUMN IF EXISTS reply,
    DROP COLUMN IF EXISTS replied_at;

DROP TABLE IF EXISTS product_reviews;
