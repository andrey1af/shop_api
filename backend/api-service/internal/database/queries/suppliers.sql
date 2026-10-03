-- name: CreateSupplier :exec
WITH new_address AS (
    INSERT INTO addresses (id, country, city, street)
    VALUES (@address_id, @country, @city, @street)
    RETURNING id
)
INSERT INTO suppliers (id, name, phone_number, address_id)
SELECT @supplier_id, @name, @phone_number, new_address.id
FROM new_address;

-- name: DeleteSupplier :one
WITH deleted_supplier AS (
    DELETE FROM suppliers
    WHERE suppliers.id = @supplier_id
    RETURNING address_id
)
DELETE FROM addresses AS a
WHERE a.id = (SELECT address_id FROM deleted_supplier)
RETURNING a.id;

-- name: GetSupplier :one
SELECT sqlc.embed(s), sqlc.embed(a)
FROM suppliers s
JOIN addresses a ON a.id = s.address_id
WHERE s.id = @supplier_id;

-- name: ListSuppliers :many
SELECT sqlc.embed(s), sqlc.embed(a)
FROM suppliers s
JOIN addresses a ON a.id = s.address_id
ORDER BY s.name, s.id;

-- name: UpdateSupplierAddress :one
UPDATE addresses AS a
SET country = @country, city = @city, street = @street
FROM suppliers s
WHERE s.id = @supplier_id AND s.address_id = a.id
RETURNING sqlc.embed(s), sqlc.embed(a);
