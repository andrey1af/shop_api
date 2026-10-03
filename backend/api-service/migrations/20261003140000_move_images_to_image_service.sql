-- +goose Up
-- Фото переехали в image-service со своими базами-шардами, в shop_api остаётся
-- только products.image_id. Старые фото из MinIO не переносятся, поэтому
-- ссылки на них обнуляются.
-- +goose StatementBegin
UPDATE products SET image_id = NULL WHERE image_id IS NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE products DROP CONSTRAINT products_image_id_fkey;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE images;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE images
(
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO images (id)
SELECT image_id FROM products WHERE image_id IS NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE products
    ADD CONSTRAINT products_image_id_fkey FOREIGN KEY (image_id) REFERENCES images (id) ON DELETE SET NULL;
-- +goose StatementEnd
