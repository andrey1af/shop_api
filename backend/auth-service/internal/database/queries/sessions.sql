-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, created_at, expires_at)
VALUES ($1, $2, $3, $4);

-- name: SessionByID :one
SELECT *
FROM sessions
WHERE id = $1;

-- name: ExtendSession :exec
UPDATE sessions
SET expires_at = $2
WHERE id = $1;

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = @revoked_at
WHERE id = @id
  AND revoked_at IS NULL;

-- name: RevokeUserSessions :exec
UPDATE sessions
SET revoked_at = @revoked_at
WHERE user_id = @user_id
  AND revoked_at IS NULL;

-- name: RevokeUserSessionsExcept :exec
UPDATE sessions
SET revoked_at = @revoked_at
WHERE user_id = @user_id
  AND id <> @keep_session_id
  AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions
WHERE expires_at <= $1;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (id, session_id, token_hash, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5);

-- name: RefreshTokenByHash :one
SELECT *
FROM refresh_tokens
WHERE token_hash = $1;

-- Помечает токен использованным, только если он ещё не был использован:
-- из двух одновременных обменов одного токена пройдёт ровно один.
-- name: MarkRefreshTokenUsed :execrows
UPDATE refresh_tokens
SET used_at = $2
WHERE id = $1
  AND used_at IS NULL;

-- name: DeleteStaleRefreshTokens :execrows
DELETE FROM refresh_tokens
WHERE expires_at <= @now
   OR used_at <= @used_before;
