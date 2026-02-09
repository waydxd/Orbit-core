-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash, first_name, last_name, email_verified, username, profile_picture, region, timezone, gender, birth_date, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: GetUserByEmail :one
SELECT id, email, password_hash, first_name, last_name, email_verified, username, profile_picture, region, timezone, gender, birth_date, created_at, updated_at
FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, password_hash, first_name, last_name, email_verified, username, profile_picture, region, timezone, gender, birth_date, created_at, updated_at
FROM users WHERE id = $1;

-- name: UpdateUser :exec
UPDATE users
SET email = $1, password_hash = $2, first_name = $3, last_name = $4, email_verified = $5, username = $6, profile_picture = $7, region = $8, timezone = $9, gender = $10, birth_date = $11, updated_at = $12
WHERE id = $13;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, email, password_hash, first_name, last_name, email_verified, username, profile_picture, region, timezone, gender, birth_date, created_at, updated_at
FROM users WHERE username = $1;

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, CURRENT_TIMESTAMP)
RETURNING id;

-- name: GetSessionByToken :one
SELECT id, user_id, token_hash, expires_at, created_at
FROM sessions WHERE token_hash = $1 AND expires_at > CURRENT_TIMESTAMP;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

