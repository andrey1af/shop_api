-- name: CreateImage :exec
INSERT INTO images (id, data, content_type)
VALUES (@id, @data, @content_type);

-- name: GetImage :one
SELECT id, data, content_type
FROM images
WHERE id = @id;

-- name: UpdateImage :one
UPDATE images
SET data = @data, content_type = @content_type, updated_at = now()
WHERE id = @id
RETURNING id;

-- name: DeleteImage :one
DELETE FROM images
WHERE id = @id
RETURNING id;
