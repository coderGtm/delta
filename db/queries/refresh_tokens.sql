-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token_hash, expires_at, revoked, user_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1 LIMIT 1;

-- name: UpdateRefreshTokenRevoked :one
UPDATE refresh_tokens SET revoked = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: RevokeAllRefreshTokensForUser :execrows
UPDATE refresh_tokens SET revoked = true, updated_at = now()
WHERE user_id = $1 AND revoked = false;

-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens WHERE expires_at < $1;

-- name: DeleteOldRevokedRefreshTokens :execrows
DELETE FROM refresh_tokens WHERE revoked = true AND updated_at < $1;