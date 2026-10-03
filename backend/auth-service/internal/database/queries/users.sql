-- name: SaveUser :one
INSERT INTO users (id, name, surname, email, phone_number, password_hash, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UserByID :one
SELECT *
FROM users
WHERE id = $1;

-- name: UserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: UpdatePassword :execrows
UPDATE users
SET password_hash = $2
WHERE id = $1;

-- name: UserByIdentity :one
SELECT sqlc.embed(users)
FROM users
JOIN user_identities ON user_identities.user_id = users.id
WHERE user_identities.provider = $1
  AND user_identities.provider_user_id = $2;

-- name: SaveUserWithIdentity :one
WITH new_user AS (
    INSERT INTO users (id, name, surname, email, phone_number, password_hash, created_at)
    VALUES (@id, @name, @surname, @email, @phone_number, @password_hash, @created_at)
    RETURNING *
), new_identity AS (
    INSERT INTO user_identities (id, user_id, provider, provider_user_id, email, created_at)
    SELECT @identity_id, new_user.id, @provider, @provider_user_id, new_user.email, new_user.created_at
    FROM new_user
)
SELECT *
FROM new_user;

-- name: SaveIdentity :exec
INSERT INTO user_identities (id, user_id, provider, provider_user_id, email, created_at)
VALUES ($1, $2, $3, $4, $5, $6);
