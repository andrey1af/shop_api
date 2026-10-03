-- name: SaveOAuthState :exec
INSERT INTO oauth_states (state, provider, code_verifier, expires_at)
VALUES ($1, $2, $3, $4);

-- name: ConsumeOAuthState :one
DELETE FROM oauth_states
WHERE state = $1
RETURNING *;

-- name: DeleteExpiredOAuthStates :exec
DELETE FROM oauth_states
WHERE expires_at <= $1;
