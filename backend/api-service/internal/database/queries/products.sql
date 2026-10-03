-- name: CreateProduct :exec
INSERT INTO products (id, name, category, price, available_stock, last_update_date, supplier_id)
VALUES (@product_id, @name, @category, @price, @available_stock, @last_update_date, @supplier_id);

-- name: DeleteProduct :one
DELETE FROM products
WHERE id = @product_id
RETURNING id;

-- name: GetProduct :one
SELECT id, name, category, price, available_stock, last_update_date, supplier_id, image_id
FROM products
WHERE id = @product_id;

-- name: ListAvailableProducts :many
SELECT id, name, category, price, available_stock, last_update_date, supplier_id, image_id
FROM products
WHERE available_stock > 0
ORDER BY name, id
LIMIT sqlc.narg('limit')::bigint OFFSET sqlc.narg('offset')::bigint;

-- name: UpdateProductStock :one
UPDATE products
SET available_stock = @available_stock, last_update_date = now()
WHERE id = @product_id
RETURNING id, name, category, price, available_stock, last_update_date, supplier_id, image_id;

-- name: ProductExistsBySupplierID :one
SELECT EXISTS(SELECT 1 FROM products WHERE supplier_id = @supplier_id);

-- name: UpdateProductPriceAndStock :one
UPDATE products
SET price = @price, available_stock = @available_stock, last_update_date = now()
WHERE id = @product_id
RETURNING id, name, category, price, available_stock, last_update_date, supplier_id, image_id;
