-- Сами фото хранятся в image-service; здесь только связь товара с фото.

-- name: LinkImageToProduct :one
UPDATE products
SET image_id = @image_id, last_update_date = now()
WHERE id = @product_id
RETURNING id;

-- name: GetImageProductID :one
SELECT id AS product_id
FROM products
WHERE image_id = @image_id;

-- name: GetImageIDByProductID :one
SELECT image_id
FROM products
WHERE id = @product_id AND image_id IS NOT NULL;

-- name: UnlinkImage :one
UPDATE products
SET image_id = NULL
WHERE image_id = @image_id
RETURNING id;
