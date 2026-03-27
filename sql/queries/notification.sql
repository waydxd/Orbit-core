-- name: UpsertDeviceToken :exec
INSERT INTO device_tokens (user_id, token, platform, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (token) DO UPDATE
  SET user_id = EXCLUDED.user_id,
      platform = EXCLUDED.platform,
      updated_at = NOW();

-- name: DeleteDeviceToken :exec
DELETE FROM device_tokens WHERE token = $1;

-- name: DeleteDeviceTokenByUser :exec
DELETE FROM device_tokens WHERE user_id = $1 AND token = $2;

-- name: GetDeviceTokensByUserID :many
SELECT id, user_id, token, platform, updated_at
FROM device_tokens
WHERE user_id = $1;

-- name: CreateSubscription :one
INSERT INTO event_subscriptions (user_id, event_id, trigger_time, is_sent, status)
VALUES ($1, $2, $3, false, 'pending')
ON CONFLICT ON CONSTRAINT idx_event_subscriptions_user_event_active
DO NOTHING
RETURNING id;

-- name: DeleteSubscription :exec
DELETE FROM event_subscriptions
WHERE user_id = $1 AND event_id = $2 AND is_sent = false;

-- name: SubscriptionExists :one
SELECT EXISTS(
  SELECT 1 FROM event_subscriptions
  WHERE user_id = $1 AND event_id = $2 AND status NOT IN ('cancelled', 'sent', 'failed')
);

-- name: GetSubscriptionByID :one
SELECT id, user_id, event_id, trigger_time, is_sent, job_id, status, created_at
FROM event_subscriptions
WHERE id = $1;

-- name: GetSubscriptionByUserAndEvent :one
SELECT id, user_id, event_id, trigger_time, is_sent, job_id, status, created_at
FROM event_subscriptions
WHERE user_id = $1 AND event_id = $2 AND status NOT IN ('cancelled', 'sent')
ORDER BY created_at DESC
LIMIT 1;

-- name: GetSubscriptionsByEventID :many
SELECT id, user_id, event_id, trigger_time, is_sent, job_id, status, created_at
FROM event_subscriptions
WHERE event_id = $1 AND status NOT IN ('cancelled', 'sent');

-- name: MarkSubscriptionStatus :exec
UPDATE event_subscriptions
SET status = $2, is_sent = $3
WHERE id = $1 AND status = 'pending';

-- name: UpdateSubscriptionJobID :exec
UPDATE event_subscriptions SET job_id = $2 WHERE id = $1;
