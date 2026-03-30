# FCM & Notification System

Orbit Core provides a push notification system for calendar event reminders using Firebase Cloud Messaging (FCM). This document explains how the components work together.

## Architecture Overview

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│  Gateway    │────▶│ Notification│────▶│    Redis    │
│  (Mobile)   │     │   (HTTP)    │     │   Service   │     │   (Asynq)   │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
                                                │                     │
                                                ▼                     ▼
                                         ┌─────────────┐     ┌─────────────┐
                                         │ PostgreSQL  │     │   Worker    │
                                         │  (Tokens,   │◀────│  (Async)    │
                                         │ Subscripts) │     └─────────────┘
                                         └─────────────┘            │
                                                                       ▼
                                                                ┌─────────────┐
                                                                │    FCM      │
                                                                │ (Firebase)  │
                                                                └─────────────┘
```

## Components

### 1. FCM Client (`pkg/fcm/fcm.go`)

Wrapper around Firebase Cloud Messaging for sending push notifications to devices.

**Key Features:**
- Singleton initialization with Firebase credentials
- Sends notification to a single device token
- Handles both Android and iOS platforms
- Detects invalid/stale tokens

**API:**
```go
// Initialize (call once at startup)
fcm.Init(ctx, credentialsJSON)

// Send notification
client.SendNotification(ctx, token, title, body, dataPayload)
```

### 2. Notification Service (`internal/notification/service.go`)

HTTP handlers for managing device tokens and event subscriptions.

**Endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/fcm/token` | Register device FCM token |
| DELETE | `/api/v1/fcm/token` | Unregister device token |
| POST | `/api/v1/events/{id}/notify` | Subscribe to event reminders |
| DELETE | `/api/v1/events/{id}/notify` | Unsubscribe from event |

**Request/Response Examples:**

**Register Token:**
```bash
POST /api/v1/fcm/token
{
  "token": "firebase_device_token",
  "platform": "ios"  # or "android"
}
# Returns: 204 No Content
```

**Subscribe to Event:**
```bash
POST /api/v1/events/123/notify
{
  "event_start_at": "2026-03-30T10:00:00Z",
  "offset_minutes": -15  # optional, default -15 minutes
}
# Returns: 201 Created
```

### 3. Asynq Worker (`internal/notification/worker.go`)

Background worker that processes queued notification tasks.

**Responsibilities:**
- Fetches pending subscriptions from the queue
- Retrieves user's device tokens from database
- Sends FCM notifications to all user devices
- Marks subscriptions as sent/failed
- Automatically removes invalid tokens

**Task Flow:**
1. Receive `notification:send` task with `{user_id, event_id, sub_id}`
2. Verify subscription status is `pending`
3. Fetch all device tokens for the user
4. Send FCM notification to each token
5. Update subscription status to `sent` (or `failed` if all sends failed)
6. Delete invalid tokens from database

### 4. Repository (`internal/notification/repository.go`)

Database operations for tokens and subscriptions using PostgreSQL.

**Data Models:**

**DeviceToken** - Stores user's FCM device tokens
```go
type DeviceToken struct {
    ID        string    // Primary key
    UserID    string    // Owner user
    Token     string    // FCM device token
    Platform  string    // "ios" or "android"
    UpdatedAt time.Time // Last updated
}
```

**EventSubscription** - Tracks reminder subscriptions
```go
type EventSubscription struct {
    ID          string    // Primary key
    UserID      string    // Subscriber
    EventID     string    // Calendar event
    TriggerTime time.Time // When to send reminder
    IsSent      bool      // Legacy flag
    JobID       *string   // Asynq task ID for cancellation
    Status      string    // pending/sent/canceled/failed
    CreatedAt   time.Time // Subscription created
}
```

## Workflows

### Device Registration Flow

1. Mobile app obtains FCM token from Firebase
2. Mobile app calls `POST /api/v1/fcm/token` with token + platform
3. Service stores/updates token in PostgreSQL
4. On notification time, worker sends to stored token

### Event Reminder Flow

1. User subscribes to event via `POST /api/v1/events/{id}/notify`
2. Service validates request (future trigger time, no duplicates)
3. Service creates subscription record in PostgreSQL
4. Service enqueues Asynq task to fire at `trigger_time`
5. At trigger time, Asynq dispatches task to worker
6. Worker fetches user's device tokens
7. Worker sends FCM notification to each token
8. Worker marks subscription as `sent`

### Unsubscribe Flow

1. User calls `DELETE /api/v1/events/{id}/notify`
2. Service fetches subscription to get Asynq job ID
3. Service cancels scheduled Asynq task
4. Service marks subscription as `canceled`

## Configuration

Required environment variables:

| Variable | Description |
|----------|-------------|
| `FIREBASE_CREDENTIALS_JSON` | Firebase service account JSON |
| `REDIS_ADDR` | Redis address for Asynq |

If Firebase credentials are not provided, notifications are skipped gracefully.

## Error Handling

- **No device tokens**: Mark as `sent` (don't retry) - user has no devices
- **All sends fail**: Mark as `failed` and allow Asynq retry
- **Partial success**: Mark as `sent` with warning log
- **Invalid token**: Auto-delete token, continue with other tokens

## Testing

Run tests:
```bash
go test ./internal/notification/...
```

Tests cover:
- Handler HTTP requests/responses
- Repository CRUD operations
- Worker task processing
- Invalid token detection and cleanup
