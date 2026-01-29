-- name: CreateLocation :exec
INSERT INTO locations (id, user_id, latitude, longitude, address, timestamp, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetLocationByID :one
SELECT id, user_id, latitude, longitude, address, timestamp, created_at
FROM locations WHERE id = $1;

-- name: GetLocationHistory :many
SELECT id, user_id, latitude, longitude, address, timestamp, created_at
FROM locations
WHERE user_id = $1
ORDER BY timestamp DESC
LIMIT $2;

-- name: GetCurrentLocation :one
SELECT id, user_id, latitude, longitude, address, timestamp, created_at
FROM locations
WHERE user_id = $1
ORDER BY timestamp DESC
LIMIT 1;

-- name: FindNearby :many
SELECT id, user_id, latitude, longitude, address, timestamp, created_at
FROM locations
WHERE ACOS(
    SIN(RADIANS(latitude)) * SIN(RADIANS(@TargetLat::float8)) +
    COS(RADIANS(latitude)) * COS(RADIANS(@TargetLat::float8)) * COS(RADIANS(longitude - @TargetLon::float8))
) * 6371 <= @Radius::float8
ORDER BY timestamp DESC
LIMIT 100;

