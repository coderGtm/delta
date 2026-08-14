-- name: GetUserByAuthUID :one
SELECT * FROM users WHERE auth_uid = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByEmailCaseInsensitive :one
SELECT * FROM users WHERE lower(email) = lower($1) AND deleted_at IS NULL LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (auth_uid, name, email, phone)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteUserRow :one
UPDATE users
SET historical_email = email, email = NULL, deleted_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;