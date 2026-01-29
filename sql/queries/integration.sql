-- name: CreateIntegration :exec
INSERT INTO integrations (id, user_id, service_name, api_key_encrypted, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetIntegrationByID :one
SELECT id, user_id, service_name, api_key_encrypted, status, last_sync, created_at, updated_at
FROM integrations WHERE id = $1;

-- name: GetIntegrationByService :one
SELECT id, user_id, service_name, api_key_encrypted, status, last_sync, created_at, updated_at
FROM integrations WHERE user_id = $1 AND service_name = $2;

-- name: ListIntegrations :many
SELECT id, user_id, service_name, api_key_encrypted, status, last_sync, created_at, updated_at
FROM integrations
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: UpdateIntegration :exec
UPDATE integrations
SET service_name = $1, api_key_encrypted = $2, status = $3, updated_at = $4
WHERE id = $5;

-- name: DeleteIntegration :exec
DELETE FROM integrations WHERE id = $1;

-- name: UpdateLastSync :exec
UPDATE integrations
SET last_sync = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

