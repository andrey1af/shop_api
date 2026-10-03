-- name: CreateClient :exec
WITH new_address AS (
    INSERT INTO addresses (id, country, city, street)
    VALUES (@address_id, @country, @city, @street)
    RETURNING id
)
INSERT INTO clients (id, client_name, client_surname, birthday, gender, registration_date, address_id)
SELECT @client_id, @client_name, @client_surname, @birthday, @gender, @registration_date, new_address.id
FROM new_address;

-- name: DeleteClient :one
WITH deleted_client AS (
    DELETE FROM clients
    WHERE clients.id = @client_id
    RETURNING address_id
)
DELETE FROM addresses AS a
WHERE a.id = (SELECT address_id FROM deleted_client)
RETURNING a.id;

-- name: FindClientsByNameAndSurname :many
SELECT sqlc.embed(c), sqlc.embed(a)
FROM clients c
JOIN addresses a ON a.id = c.address_id
WHERE c.client_name = @client_name AND c.client_surname = @client_surname;

-- name: ListClients :many
SELECT sqlc.embed(c), sqlc.embed(a)
FROM clients c
JOIN addresses a ON a.id = c.address_id
ORDER BY c.registration_date, c.id
LIMIT sqlc.narg('limit')::bigint OFFSET sqlc.narg('offset')::bigint;

-- name: UpdateClientAddress :one
UPDATE addresses AS a
SET country = @country, city = @city, street = @street
FROM clients c
WHERE c.id = @client_id AND c.address_id = a.id
RETURNING sqlc.embed(c), sqlc.embed(a);
