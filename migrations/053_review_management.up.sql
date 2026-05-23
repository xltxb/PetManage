-- Add images and merchant reply columns to service_evaluations
ALTER TABLE service_evaluations
    ADD COLUMN IF NOT EXISTS images TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reply TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS replied_at TIMESTAMPTZ;

-- Product reviews: customer ratings and reviews for purchased products
CREATE TABLE IF NOT EXISTS product_reviews (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL,
    member_id BIGINT NOT NULL,
    order_item_id BIGINT NOT NULL,
    product_id BIGINT NOT NULL,
    rating INT NOT NULL DEFAULT 5,
    content TEXT NOT NULL DEFAULT '',
    images TEXT NOT NULL DEFAULT '',
    reply TEXT NOT NULL DEFAULT '',
    replied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_product_review_rating CHECK (rating >= 1 AND rating <= 5)
);

CREATE INDEX IF NOT EXISTS idx_product_reviews_product ON product_reviews(product_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_product_reviews_merchant ON product_reviews(merchant_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_product_reviews_member ON product_reviews(member_id, deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_reviews_unique ON product_reviews(order_item_id);
