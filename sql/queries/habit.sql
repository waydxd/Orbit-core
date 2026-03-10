-- name: UpsertEventFrequency :one
INSERT INTO event_frequency (
    id, user_id, title, description, location, duration_minutes,
    time_of_day, day_of_week, occurrence_count, suggestion_threshold,
    suggestion_shown, habit_accepted, occurrence_timestamps, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (user_id, title, duration_minutes, time_of_day, day_of_week)
DO UPDATE SET
    occurrence_count = event_frequency.occurrence_count + 1,
    occurrence_timestamps = $13,
    updated_at = $15
RETURNING id, occurrence_count;

-- name: GetEventFrequencyByPattern :one
SELECT id, user_id, title, description, location, duration_minutes,
       time_of_day, day_of_week, occurrence_count, suggestion_threshold,
       suggestion_shown, habit_accepted, occurrence_timestamps, created_at, updated_at
FROM event_frequency
WHERE user_id = $1 AND title = $2 AND duration_minutes = $3
      AND time_of_day = $4 AND day_of_week = $5;

-- name: GetEventFrequenciesAboveThreshold :many
SELECT id, user_id, title, description, location, duration_minutes,
       time_of_day, day_of_week, occurrence_count, suggestion_threshold,
       suggestion_shown, habit_accepted, occurrence_timestamps, created_at, updated_at
FROM event_frequency
WHERE user_id = $1 AND occurrence_count >= $2 AND suggestion_shown = FALSE AND habit_accepted = FALSE
ORDER BY occurrence_count DESC;

-- name: UpdateEventFrequency :exec
UPDATE event_frequency
SET occurrence_count = $1, suggestion_shown = $2, habit_accepted = $3,
    occurrence_timestamps = $4, updated_at = $5
WHERE id = $6;

-- name: CreateHabitSuggestion :exec
INSERT INTO habit_suggestions (
    id, user_id, event_frequency_id, title, description, location,
    duration_minutes, time_of_day, day_of_week, status, created_at, updated_at, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: GetPendingHabitSuggestions :many
SELECT id, user_id, event_frequency_id, title, description, location,
       duration_minutes, time_of_day, day_of_week, status,
       recurrence_end_date, created_at, updated_at, expires_at
FROM habit_suggestions
WHERE user_id = $1 AND status = 'pending' AND expires_at > NOW()
ORDER BY created_at DESC;

-- name: GetHabitSuggestionByID :one
SELECT id, user_id, event_frequency_id, title, description, location,
       duration_minutes, time_of_day, day_of_week, status,
       recurrence_end_date, created_at, updated_at, expires_at
FROM habit_suggestions
WHERE id = $1;

-- name: UpdateHabitSuggestionStatus :exec
UPDATE habit_suggestions
SET status = $1, recurrence_end_date = $2, updated_at = $3
WHERE id = $4;

-- name: CreateRecurringEvent :exec
INSERT INTO events (
    id, user_id, title, description, location,
    start_time, end_time, is_recurring, recurrence_rule, recurrence_exception, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: GetActiveRecurringEvents :many
SELECT id, user_id, title, description, location, hashtags,
       start_time, end_time, is_recurring, recurrence_rule, recurrence_exception,
       created_at, updated_at
FROM events
WHERE user_id = $1 AND is_recurring = TRUE
ORDER BY created_at DESC;

-- name: DeactivateRecurringEvent :exec
UPDATE events
SET is_recurring = FALSE, updated_at = $1
WHERE id = $2;

