-- name: CreateEvent :exec
INSERT INTO events (id, user_id, title, description, start_time, end_time, location, hashtags, is_recurring, recurrence_rule, recurrence_exception, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: GetEventByID :one
SELECT id, user_id, title, description, start_time, end_time, location, hashtags, is_recurring, recurrence_rule, recurrence_exception, created_at, updated_at
FROM events WHERE id = $1;

-- name: ListEventsByTime :many
SELECT e.id, e.user_id, e.title, e.description, e.start_time, e.end_time, e.location, e.hashtags, e.is_recurring, e.recurrence_rule, e.recurrence_exception, e.created_at, e.updated_at
FROM events e
WHERE e.start_time <= sqlc.arg('window_end') AND e.end_time >= sqlc.arg('window_start')
UNION
SELECT e.id, e.user_id, e.title, e.description, e.start_time, e.end_time, e.location, e.hashtags, e.is_recurring, e.recurrence_rule, e.recurrence_exception, e.created_at, e.updated_at
FROM events e
WHERE e.is_recurring = true
  AND e.start_time <= sqlc.arg('window_end')
ORDER BY start_time;

-- name: ListEventsByUserAndTime :many
SELECT e.id, e.user_id, e.title, e.description, e.start_time, e.end_time, e.location, e.hashtags, e.is_recurring, e.recurrence_rule, e.recurrence_exception, e.created_at, e.updated_at
FROM events e
WHERE e.user_id = sqlc.arg('user_id')
  AND e.start_time <= sqlc.arg('window_end') AND e.end_time >= sqlc.arg('window_start')
UNION
SELECT e.id, e.user_id, e.title, e.description, e.start_time, e.end_time, e.location, e.hashtags, e.is_recurring, e.recurrence_rule, e.recurrence_exception, e.created_at, e.updated_at
FROM events e
WHERE e.user_id = sqlc.arg('user_id')
  AND e.is_recurring = true
  AND e.start_time <= sqlc.arg('window_end')
ORDER BY start_time;

-- name: UpdateEvent :exec
UPDATE events
SET title = $1, description = $2, start_time = $3, end_time = $4, location = $5, hashtags = $6, is_recurring = $7, recurrence_rule = $8, recurrence_exception = $9, updated_at = $10
WHERE id = $11;

-- name: DeleteEvent :exec
DELETE FROM events WHERE id = $1;

-- name: CreateTask :exec
INSERT INTO tasks (id, user_id, title, description, due_date, completed, priority, hashtags, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: GetTaskByID :one
SELECT id, user_id, title, description, due_date, completed, priority, hashtags, created_at, updated_at
FROM tasks WHERE id = $1;

-- name: ListTasks :many
SELECT id, user_id, title, description, due_date, completed, priority, hashtags, created_at, updated_at
FROM tasks
WHERE user_id = $1
  AND (sqlc.narg('completed')::boolean IS NULL OR completed = sqlc.narg('completed'))
ORDER BY due_date ASC;

-- name: UpdateTask :exec
UPDATE tasks
SET title = $1, description = $2, due_date = $3, completed = $4, priority = $5, hashtags = $6, updated_at = $7
WHERE id = $8;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id = $1;

