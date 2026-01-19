-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash, first_name, last_name, email_verified, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetUserByEmail :one
SELECT id, email, password_hash, first_name, last_name, email_verified, created_at, updated_at
FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, password_hash, first_name, last_name, email_verified, created_at, updated_at
FROM users WHERE id = $1;

-- name: UpdateUser :exec
UPDATE users
SET email = $1, password_hash = $2, first_name = $3, last_name = $4, email_verified = $5, updated_at = $6
WHERE id = $7;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, CURRENT_TIMESTAMP)
RETURNING id;

-- name: GetSessionByToken :one
SELECT id, user_id, token_hash, expires_at, created_at
FROM sessions WHERE token_hash = $1 AND expires_at > CURRENT_TIMESTAMP;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

