# Chat Service Implementation

## Overview

The Chat Service provides a complete backend implementation for an AI-powered chatbot with action confirmation workflows. It implements all the requirements from the Calendar Backend Implementation Todo List.

## Features Implemented

### Phase 2: Core REST API Endpoints

1. **POST /api/v1/chat/messages**
   - Validates user requests
   - Generates correlation_id for distributed tracing
   - Creates or retrieves conversations
   - Forwards messages to Agent Runner (placeholder for gRPC integration)
   - Persists user messages and agent replies
   - Stores proposed actions for user confirmation

2. **GET /api/v1/chat/conversations/{conversation_id}**
   - Returns complete conversation history
   - Lists all pending actions
   - Shows conversation status

3. **POST /api/v1/chat/actions/{action_id}/confirm**
   - Validates idempotency key
   - Checks action status and expiration
   - Re-validates action against business rules
   - Detects conflicts
   - Executes action via gRPC
   - Updates action status with optimistic locking

4. **POST /api/v1/chat/actions/{action_id}/cancel**
   - Cancels pending actions
   - Logs audit trail

5. **GET /api/v1/chat/actions/{action_id}**
   - Returns action details and status

6. **GET /api/v1/chat/metrics**
   - Returns real-time metrics snapshot

### Phase 3: Pending Action Model and Persistence

- **Database Schema**: Complete schema with conversations, chat_messages, pending_actions, and agent_tool_logs tables
- **Optimistic Locking**: Version-based locking prevents double execution
- **TTL Cleanup**: Background job expires stale pending actions
- **Audit Logging**: Agent tool calls linked to pending actions

### Phase 4: gRPC Integration

- **Calendar Service Integration**: Executes create, update, and delete operations
- **Idempotency Support**: All write operations use idempotency keys
- **Operation IDs**: Stable operation IDs returned for tracking

### Phase 5: Business Logic and Policies

- **Propose vs Execute**: Agent proposes, user confirms - enforced separation
- **Policy Validator**: 
  - Event duration limits (15min - 8hrs)
  - Attendee limits (max 50)
  - Past event prevention
  - Bulk operation blocking
- **Conflict Detection**: Framework for detecting overlapping events
- **Validation on Confirmation**: Re-validates before execution

### Phase 6: Reliability and Observability

- **Correlation IDs**: Generated and propagated across all operations
- **Structured Logging**: Comprehensive logging with context
- **Metrics Tracking**:
  - Message and conversation counts
  - Pending/confirmed/cancelled/expired action counts
  - Average latencies for messages and actions
  - Error rates and types
  - Confirmation and success rates
  - Per-minute rates

### Phase 8: Error Handling

- **User-Friendly Errors**: Clear error codes and messages
- **HTTP Status Codes**: Proper use of 400, 404, 409, 410, 500
- **Idempotency**: Prevents duplicate executions

### Phase 10: Documentation

- **OpenAPI Spec**: Complete API documentation
- **Example Payloads**: Request/response examples for all endpoints

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────┐
│   Gateway (Rate Limiting)    │
└──────────┬──────────────────┘
           │
           ▼
    ┌──────────────┐
    │ Chat Service │
    └──────┬───────┘
           │
    ┌──────┴───────────────┬──────────────┬─────────────┐
    ▼                      ▼              ▼             ▼
┌────────┐          ┌──────────┐    ┌─────────┐   ┌─────────┐
│MongoDB │          │ gRPC     │    │ Policy  │   │ Metrics │
│(Chat)  │          │ Client   │    │Validator│   │ Tracker │
└────────┘          └──────────┘    └─────────┘   └─────────┘
                         │
                         ▼
                  ┌─────────────┐
                  │Agent Runner │
                  └─────────────┘
```

## Database Schema

### conversations
- id (UUID, PK)
- user_id (FK to users)
- correlation_id (UUID, unique)
- status (active/closed/archived)
- created_at, updated_at

### chat_messages
- id (UUID, PK)
- conversation_id (FK)
- user_id (FK)
- role (user/assistant/system)
- content (TEXT)
- metadata (JSONB)
- created_at

### pending_actions
- id (UUID, PK)
- action_id (VARCHAR, unique)
- user_id (FK)
- conversation_id (FK)
- proposed_action (JSONB)
- action_type (VARCHAR)
- idempotency_key (VARCHAR, unique)
- status (pending/confirmed/cancelled/expired/failed)
- version (INT) - for optimistic locking
- correlation_id (UUID)
- agent_metadata (JSONB)
- error_message (TEXT)
- created_at, updated_at, expires_at

### agent_tool_logs
- id (UUID, PK)
- pending_action_id (FK, nullable)
- conversation_id (FK)
- user_id (FK)
- tool_name (VARCHAR)
- tool_input (JSONB)
- tool_output (JSONB)
- status (started/success/failed)
- error_message (TEXT)
- correlation_id (UUID)
- created_at

## Policy Rules

1. **Event Duration**
   - Minimum: 15 minutes
   - Maximum: 8 hours

2. **Attendees**
   - Maximum: 50 per event

3. **Time Constraints**
   - Cannot create events more than 1 hour in the past
   - End time must be after start time

4. **Bulk Operations**
   - Mass deletes are blocked
   - Only single-item operations allowed

5. **Action Expiry**
   - Pending actions expire after 24 hours
   - Cleanup job runs every 5 minutes

## Metrics Available

- `total_messages`: Total chat messages processed
- `total_conversations`: Total conversations created
- `total_pending_actions`: Total actions proposed
- `total_confirmed_actions`: Total actions confirmed
- `total_cancelled_actions`: Total actions cancelled
- `total_expired_actions`: Total actions expired
- `total_failed_actions`: Total actions that failed
- `avg_message_latency_ms`: Average message processing time
- `avg_action_latency_ms`: Average action execution time
- `confirmation_rate_pct`: Percentage of confirmed actions
- `success_rate_pct`: Percentage of successful executions
- `total_errors`: Total errors
- `validation_errors`: Validation failures
- `policy_violations`: Policy rule violations
- `conflict_errors`: Conflict detection hits
- `messages_per_minute`: Message rate
- `actions_per_minute`: Action creation rate

## Usage Examples

### Send a Message

```bash
curl -X POST http://localhost:8080/api/v1/chat/messages \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Schedule a meeting with John tomorrow at 2pm",
    "user_id": "user-123"
  }'
```

### Get Conversation

```bash
curl http://localhost:8080/api/v1/chat/conversations/conv-uuid-123
```

### Confirm Action

```bash
curl -X POST http://localhost:8080/api/v1/chat/actions/action-uuid-789/confirm \
  -H "Content-Type: application/json" \
  -d '{
    "idempotency_key": "idem-key-xyz"
  }'
```

### Cancel Action

```bash
curl -X POST http://localhost:8080/api/v1/chat/actions/action-uuid-789/cancel
```

### Get Metrics

```bash
curl http://localhost:8080/api/v1/chat/metrics
```

## Security Considerations

1. **Idempotency**: All write operations use idempotency keys to prevent duplicates
2. **Optimistic Locking**: Version numbers prevent race conditions
3. **Policy Validation**: Business rules enforced at multiple layers
4. **Audit Logging**: All agent actions logged for compliance
5. **Expiration**: Actions automatically expire to prevent stale operations
6. **Correlation IDs**: Enable end-to-end request tracing

## Future Enhancements

1. **Retry/Backoff**: Add exponential backoff for transient gRPC failures
2. **Circuit Breaker**: Implement circuit breaker pattern for gRPC calls
3. **Token Propagation**: Add capability tokens to gRPC metadata
4. **Advanced Conflict Detection**: Query calendar service for overlapping events
5. **Notification Hooks**: Notify users when actions expire or fail
6. **Rate Limiting**: Add per-user rate limits on actions
7. **Metrics Export**: Export metrics to Prometheus/Grafana
8. **Distributed Tracing**: Integrate OpenTelemetry for full traces

## Testing

To run the application:

```bash
# Build
make build

# Run
./bin/orbit-core

# Or directly
go run cmd/orbit-core/main.go
```

The service will be available at `http://localhost:8080/api/v1/chat/*`
