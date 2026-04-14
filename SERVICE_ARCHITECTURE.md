# 2.1.3.1 Service Architecture

The Orbit-core system is architected as a **modular monolith** with specialized, independent services working together through well-defined contracts. This design improves modularity, scalability, and maintainability while allowing future evolution into a microservices architecture.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Client Applications                          │
│                     (Web, Mobile, Desktop)                           │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTP/REST
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    GATEWAY SERVICE (API Router)                      │
│  • Request routing & orchestration                                   │
│  • Authentication middleware                                         │
│  • Rate limiting (Redis-backed)                                      │
│  • Health check endpoint                                             │
└─┬──────────┬─────────────┬─────────────┬──────────────┬────────────────┤
  │          │             │             │              │                │
  ▼          ▼             ▼             ▼              ▼                ▼
┌──────┐ ┌───────┐ ┌──────────┐ ┌─────────┐ ┌────────┐ ┌───────┐ ┌────────┐
│ AUTH │ │CALENDAR│ │LOCATION  │ │INTEGRATION│ │CHAT   │ │ HABIT │ │ASSET  │
│      │ │ TASK  │ │          │ │           │ │       │ │      │ │       │
└──────┘ └───────┘ └──────────┘ └─────────┘ └───────┘ └───────┘ └───────┘
    │        │         │             │            │           │          │
    └────────┴─────────┴─────────────┴───────────┴───────────┴──────────┘
                       │ SQL Queries
                       ▼
              ┌──────────────────┐
              │   PostgreSQL   │
              │   (Primary DB)  │
              └──────────────────┘
              
    Additional Infrastructure:
              ┌──────────────────┐
              │     MongoDB      │ (Chat data, sessions)
              │  (Document DB)   │
              └──────────────────┘
              
              ┌──────────────────┐
              │     Redis        │ (Rate limiting, caching)
              │  (Cache/Queue)   │
              └──────────────────┘
              
              ┌──────────────────┐
              │  gRPC Server     │ (Calendar Data Service)
              │  (Port 50051)    │
              └──────────────────┘
              
              ┌──────────────────┐
              │     Asynq        │ (Task queue for notifications)
              │  (Background)   │
              └──────────────────┘
```

## Core Services

### 1. **Gateway Service**
**Purpose**: Acts as the single entry point for all client requests, orchestrating traffic across the system.

**Key Responsibilities**:
- Request routing to appropriate microservices
- HTTP endpoint management and health checking
- Authentication middleware enforcement
- Rate limiting using Redis (100 requests/minute per client)
- CORS and request validation

**Interfaces Defined**:
```
AuthServiceInterface
CalendarServiceInterface
LocationServiceInterface
IntegrationServiceInterface
ChatServiceInterface
HabitServiceInterface
NotificationServiceInterface
AssetServiceInterface
```

**Technology**: 
- Gorilla Mux (HTTP router)
- Redis (rate limiting)
- JWT middleware (authentication)

**Port**: `8080` (configurable)

---

### 2. **Authentication Service**
**Purpose**: Manages secure user verification, session handling, and credential management.

**Key Responsibilities**:
- User registration and login
- Email verification workflow
- Password reset functionality
- JWT token generation (default: 24-hour expiration)
- Password hashing with Argon2id algorithm
- User profile management
- Session management via Redis

**HTTP Endpoints**:
```
POST   /auth/register                  - Create new user account
POST   /auth/login                     - Authenticate and generate JWT
POST   /auth/verify                    - Verify JWT token validity
POST   /auth/logout                    - Invalidate token
POST   /auth/password-reset-request    - Initiate password reset flow
POST   /auth/password-reset-confirm    - Confirm password reset
GET    /auth/verify-email              - Email verification link handler
GET    /profile                        - Get user profile (protected)
PUT    /profile                        - Update user profile (protected)
```

**Security Features**:
- **Password Hashing**: Argon2id with configurable parameters (1 iteration, 64MB memory, 4 parallelism)
- **Token Management**: SHA-256 hashing for token storage in Redis
- **Timing Attack Mitigation**: Constant-time comparison for password verification
- **Email Verification**: Token-based email verification with Resend integration
- **Password Reset**: Secure token-based password reset flow

**Data Storage**:
- **PostgreSQL**: User credentials, profile data
- **Redis**: Session tokens, verification tokens (TTL-based)

**External Dependencies**:
- Resend API (email delivery)
- PostgreSQL (user data)
- Redis (session management)

---

### 3. **Calendar & Task Service**
**Purpose**: Provides core scheduling and event management functionality with support for recurring events.

**Key Responsibilities**:
- Event creation, retrieval, updates, and deletion
- Task management (CRUD operations)
- Recurring event support (daily, weekly, monthly, yearly)
- Time range filtering for event queries
- Event status tracking (scheduled, completed, cancelled)

**HTTP Endpoints**:
```
GET    /calendar/events                    - List events (with time filter)
POST   /calendar/events                    - Create new event
GET    /calendar/events/{id}               - Get event details
PUT    /calendar/events/{id}               - Update event
DELETE /calendar/events/{id}               - Delete event

GET    /calendar/tasks                     - List tasks
POST   /calendar/tasks                     - Create new task
GET    /calendar/tasks/{id}                - Get task details
PUT    /calendar/tasks/{id}                - Update task
DELETE /calendar/tasks/{id}                - Delete task
```

**gRPC Services** (for Agent integration):
- `CalendarDataService` - Read-only access to calendar data
- `CalendarService` - Full CRUD operations for calendar events

**Features**:
- **Recurring Events**: Supports daily, weekly, monthly, and yearly recurrence patterns
- **Time Range Queries**: Default 3-month window (previous, current, next month)
- **Event Status**: Tracks event lifecycle (scheduled, completed, cancelled)
- **Adapter Pattern**: Provides adapters for external service integration

**Data Storage**:
- **PostgreSQL**: Events, tasks, recurrence rules, audit logs
- **SQL Migrations**: 5 schema versions with support for recurring events and indexing

---

### 4. **Location Service**
**Purpose**: Manages location-based features, tracking, and geolocation functionality.

**Key Responsibilities**:
- Location tracking and history management
- Current location retrieval
- Proximity-based location discovery (nearby places)
- Geospatial queries

**HTTP Endpoints**:
```
POST   /location/track                 - Record user location
GET    /location/history               - Retrieve location history
GET    /location/current               - Get current location
GET    /location/nearby                - Find nearby locations/places
```

**Request Model**:
```json
{
  "latitude": 40.7128,
  "longitude": -74.0060,
  "address": "Optional address string"
}
```

**Data Storage**:
- **PostgreSQL**: Location records with timestamp, latitude, longitude, and address

---

### 5. **Integration Service**
**Purpose**: Handles integration with external APIs and calendar systems (Google Calendar, CSV, ICS formats).

**Key Responsibilities**:
- Google Calendar integration (authentication, sync, import/export)
- Calendar export to multiple formats (CSV, ICS)
- Calendar import from external sources
- OAuth 2.0 flow for Google Calendar
- Format conversion utilities

**HTTP Endpoints**:
```
GET    /integration/google/auth           - Initiate Google Calendar OAuth
GET    /integration/google/callback       - Handle OAuth callback
POST   /integration/google/sync           - Sync with Google Calendar
POST   /integration/import                - Import calendar data
POST   /integration/export                - Export calendar data
GET    /integration/formats               - List supported formats
```

**Supported Formats**:
- **ICS** (iCalendar format)
- **CSV** (Comma-separated values)
- **Google Calendar** (via OAuth 2.0)

**Key Features**:
- **In-Memory Token Store** (development only - requires database-backed implementation for production)
- **Service Composition**: Depends on Calendar Service for read/write operations
- **Format Validation**: Validates imported data before storing

**External Dependencies**:
- Google Calendar API
- PostgreSQL (data persistence)

---

### 6. **Habit Tracking Service**
**Purpose**: Automatically detects and suggests recurring habits based on user behavior.

**Key Responsibilities**:
- Event pattern analysis for recurring behavior detection
- Habit suggestion generation (3+ occurrences triggers suggestion)
- Suggestion management (accept/reject)
- Auto-scheduling of accepted habits (5-year schedule)

**HTTP Endpoints**:
```
GET    /habit/suggestions                   - Get habit suggestions
POST   /habit/suggestions/{id}/accept     - Accept habit suggestion
POST   /habit/suggestions/{id}/reject     - Reject habit suggestion
```

**Detection Criteria**:
- Same event title (case-insensitive)
- Same day of week
- Same time (+/- 1 hour window)
- Same duration (+/- 15 min window)
- Minimum 3 occurrences

**Features**:
- **Pattern Recognition**: Analyzes user event history for recurring patterns
- **Smart Scheduling**: Automatically creates 5-year recurring event on accept
- **Decline Tracking**: Stores rejected suggestions to avoid repeat prompts

**Data Storage**:
- **PostgreSQL**: Habit suggestions, detection rules

---

### 7. **Notification Service**
**Purpose**: Handles push notifications and event reminders using Firebase Cloud Messaging.

**Key Responsibilities**:
- Device token registration (iOS/Android)
- Push notification delivery via FCM
- Event reminder scheduling
- Notification preferences management
- Background task processing via Asynq

**HTTP Endpoints**:
```
POST   /fcm/token                    - Register device token
DELETE /fcm/token                    - Delete device token
POST   /events/{id}/notify           - Subscribe to event notifications
DELETE /events/{id}/notify           - Unsubscribe from notifications
```

**Request Models**:
```json
// Register Token
{
  "token": "device-fcm-token",
  "device_type": "ios"  // or "android"
}
```

**Key Features**:
- **FCM Integration**: Firebase Cloud Messaging for push notifications
- **Asynq Queue**: Background task scheduling for reminders
- **Event Subscriptions**: Per-event notification preferences
- **Multi-Device**: Support for multiple devices per user

**Data Storage**:
- **PostgreSQL**: FCM tokens, notification subscriptions
- **Asynq**: Scheduled reminder tasks

---

### 8. **Asset Service**
**Purpose**: Manages binary asset uploads including event images and user profile pictures.

**Key Responsibilities**:
- Event image upload and storage
- User profile picture management
- Image validation (magic bytes detection)
- Asset serving with access control

**HTTP Endpoints**:
```
POST   /events/{id}/images              - Upload event image
GET    /events/{id}/images              - List event images
POST   /users/me/profile-pic          - Upload profile picture
GET    /assets/events/{image_id}        - Serve event image
GET    /assets/users/{image_id}          - Serve user avatar
```

**Features**:
- **Magic Byte Validation**: Validates actual image format using file headers
- **Supported Formats**: JPEG, PNG, WebP
- **File Size Limit**: 5 MB per image
- **Event Image Limit**: 5 images per event
- **Access Control**: Users can only access their own assets
- **Profile Pictures**: Profile pics are publicly readable

**Data Storage**:
- **PostgreSQL**: Asset metadata (URLs, associations)
- **Local Filesystem**: Binary image storage

---

### 9. **Chat Service**
**Purpose**: Provides chatbot backend with action confirmation workflows and state management.

**Key Responsibilities**:
- Multi-turn conversation management
- Pending action tracking and confirmation
- Action expiration (24-hour default TTL)
- Conversation history persistence
- Action policy validation
- Automatic cleanup of expired actions

**HTTP Endpoints**:
```
POST   /chat/messages                           - Send message
GET    /chat/conversations/{conversation_id}   - Get conversation
GET    /chat/actions/{action_id}                - Get action details
POST   /chat/actions/{action_id}/confirm        - Confirm pending action
POST   /chat/actions/{action_id}/cancel         - Cancel pending action
GET    /chat/metrics                            - Get service metrics
```

**Request Models**:
```json
// Post Message
{
  "message": "User message",
  "conversation_id": "optional-id",
  "context": { "key": "value" }
}

// Confirm Action
{
  "idempotency_key": "unique-key"
}
```

**Response Model**:
```json
{
  "conversation_id": "conv-uuid",
  "reply": "Bot response",
  "proposed_action_summary": "Summary of action",
  "action_id": "action-uuid",
  "correlation_id": "correlation-uuid",
  "metadata": { "key": "value" }
}
```

**Key Features**:
- **Idempotency**: Idempotency keys prevent duplicate action execution
- **Action Lifecycle**: Proposed → Pending → Confirmed/Cancelled → Executed/Expired
- **Policy Validation**: Custom validators for action authorization
- **Automatic Cleanup**: Cleanup job removes expired actions every 5 minutes
- **Metrics Tracking**: Tracks action counts by type and status

**Data Storage**:
- **MongoDB**: Conversations, messages, pending actions, action history
- **Document Schema**: Flexible document structure for chat data

**Action Expiration**: Default 24 hours (configurable via config)

---

## Service Dependencies & Communication

### Dependency Graph

```
                          ┌──────────────┐
                          │   Gateway    │
                          │   Service    │
                          └──────┬───────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
         ▼                       ▼                       ▼
     ┌────────┐            ┌────────────┐          ┌──────────┐
     │  Auth  │            │ Calendar   │          │Location  │
     │Service │            │   Service  │          │Service   │
     └────────┘            └─────┬──────┘          └──────────┘
                                 │
                     ┌───────────┼───────────┐
                     │           │           │
                     ▼           ▼           ▼
                 ┌────────┐  ┌──────────┐  ┌───────┐
                 │ Chat   │  │Integration│ │Habit │
                 │Service │  │  Service  │ │Service│
                 └────────┘  └──────────┘  └───────┘
                     │           │
         ┌───────────┴───────────┤
         │                       │
         ▼                       ▼
     ┌────────┐            ┌──────────┐
     │Notification│       │  Asset   │
     │ Service    │        │ Service  │
     └────────────┘        └─────────┘
             │                       │
             └───────────┬───────────┘
                         │
                  ┌──────▼──────┐
                  │   gRPC      │
                  │   Server   │
                  └────────────┘
```

### Data Flow Patterns

#### Authentication Flow
```
Client
  ↓
Gateway (Rate Limit Check)
  ↓
Auth Service
  ├─ Validate credentials
  ├─ Argon2id password verification
  ├─ Generate JWT token
  └─ Store in Redis (with TTL)
  ↓
Return JWT Token
```

#### Calendar Operations Flow
```
Client Request
  ↓
Gateway (Route & Auth Middleware)
  ↓
Calendar Service
  ├─ SQL query to PostgreSQL
  ├─ Apply business logic
  └─ Return data
  ↓
Response to Client
```

#### Agent Interaction Flow
```
User Prompt
  ↓
Gateway
  ↓
Agent Service
  ├─ Retrieve calendar context
  ├─ Send to gRPC Orbi Agent
  ├─ Process AI response
  └─ Execute calendar operations
  ↓
Chat Service (Action Tracking)
  ├─ Store pending action
  ├─ Wait for confirmation
  └─ Execute on confirm
  ↓
Response with Action Summary
```

---

## Infrastructure & Persistence

### Database Architecture

```
┌─────────────────────────────────────────────┐
│           Application Layer                  │
│  (Auth, Calendar, Location, Integration)    │
└──────────────┬──────────────────────────────┘
               │
       ┌───────┴─────────┐
       │                 │
       ▼                 ▼
  ┌─────────────┐   ┌──────────────┐
  │ PostgreSQL  │   │   MongoDB    │
  │  (SQL DB)   │   │  (Document)  │
  └─────────────┘   └──────────────┘
       │                 │
  ┌────┴────┐       ┌────┴────┐
  │  Auth   │       │  Chat   │
  │ Calendar│       │  Data   │
  │Location │       └─────────┘
  └─────────┘

Additional Infrastructure:
┌─────────────────────┐
│      Redis          │
├─────────────────────┤
│ • Session Tokens    │
│ • Rate Limiting     │
│ • Caching Layer     │
└─────────────────────┘

┌─────────────────────┐
│    gRPC Server      │
├─────────────────────┤
│ • Port 50051        │
│ • CalendarService   │
│ • Orbi Integration  │
└─────────────────────┘
```

### Database Schemas

**PostgreSQL Tables** (via migrations):
- `users` - User accounts and authentication
- `user_profiles` - Extended user profile information
- `events` - Calendar events with recurrence support
- `tasks` - Task management data
- `locations` - Location tracking records
- `integrations` - External service credentials/state
- `habit_suggestions` - Detected habit patterns
- `fcm_tokens` - Firebase Cloud Messaging tokens
- `notification_subscriptions` - Event notification preferences

**MongoDB Collections**:
- `conversations` - Chat conversation records
- `messages` - Chat messages with metadata
- `pending_actions` - Actions awaiting user confirmation
- `action_history` - Completed/expired action audit trail

---

## Configuration & Environment

Services are configured via environment variables and config files:

```yaml
# Database Configuration
DATABASE_URL=postgres://user:pass@host/db
MONGODB_URI=mongodb://host:port/db

# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Authentication
JWT_SECRET=your-secret-key
JWT_EXPIRATION=24  # hours
RESEND_API_KEY=optional-email-service

# Server Configuration
SERVER_PORT=8080
GRPC_SERVER_PORT=50051

# External Services
GRPC_AGENT_HOST=localhost
GRPC_AGENT_PORT=50051
GOOGLE_OAUTH_CLIENT_ID=your-client-id
GOOGLE_OAUTH_CLIENT_SECRET=your-secret
```

---

## Design Patterns

### 1. **Repository Pattern**
Each service implements repositories for data access:
```go
type Repository interface {
    Create(ctx context.Context, entity Entity) error
    Read(ctx context.Context, id string) (Entity, error)
    Update(ctx context.Context, entity Entity) error
    Delete(ctx context.Context, id string) error
}
```

### 2. **Adapter Pattern**
Services expose adapters for external integration:
```go
func (s *Service) ListEventsAdapter(ctx context.Context, startTime, endTime int64) ([]interface{}, error)
func (s *Service) CreateEventAdapter(ctx context.Context, event interface{}) (interface{}, error)
```

### 3. **Interface-Based Design**
Services define clear interfaces for dependency injection:
```go
type AuthServiceInterface interface {
    RegisterRoutes(router *mux.Router)
    RegisterProtectedRoutes(router *mux.Router)
}
```

### 4. **Middleware Pattern**
Authentication and rate limiting via HTTP middleware:
```go
authMiddleware := middleware.NewAuthMiddleware(jwtSecret)
rateLimiter := middleware.NewRateLimiter(redisAddr, 100, 1*time.Minute)
```

---

## Scalability & Evolution

### Current Architecture (Monolith)
- All services in single Go process
- Shared PostgreSQL database
- Easier local development
- Rapid prototyping

### Future Microservices Evolution

```
                    ┌──────────┐
                    │ API      │
                    │Gateway   │
                    └────┬─────┘
        ┌───────────────┼───────────────┐
        │               │               │
        ▼               ▼               ▼
    ┌────────┐     ┌────────────┐   ┌──────────┐
    │Auth    │     │ Calendar   │   │Location  │
    │Service │     │ Service    │   │Service   │
    │(Micro) │     │(Micro)     │   │(Micro)   │
    └────────┘     └────────────┘   └──────────┘
        │               │               │
    ┌───┴───────────────┴───────────────┴──┐
    │      Service Mesh (Optional)         │
    │      - gRPC Communication            │
    │      - Service Discovery             │
    │      - Load Balancing                │
    └──────────────────────────────────────┘
```

---

## Deployment Model

### Container Architecture

```dockerfile
# Single Container (Monolith)
FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go build -o orbit-core cmd/orbit-core/main.go
EXPOSE 8080 50051
CMD ["./orbit-core"]
```

### Docker Compose (Development)

```yaml
services:
  postgres:
    image: postgres:15
    ports: ["5432:5432"]
    environment:
      POSTGRES_DB: orbit
      POSTGRES_USER: postgres

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  mongodb:
    image: mongo:6
    ports: ["27017:27017"]

  orbit-core:
    build: .
    ports: ["8080:8080", "50051:50051"]
    depends_on: [postgres, redis, mongodb]
    environment:
      DATABASE_URL: postgres://postgres:postgres@postgres/orbit
      REDIS_ADDR: redis:6379
      MONGODB_URI: mongodb://mongodb:27017/orbit
```

---

## Security Considerations

### Authentication & Authorization
- **JWT Tokens**: 24-hour expiration (configurable)
- **Password Hashing**: Argon2id with salting
- **Token Storage**: Secure Redis with TTL
- **Timing Attack Mitigation**: Constant-time comparison

### Data Protection
- **PostgreSQL SSL**: Optional TLS for database connections
- **Redis Password**: Optional authentication
- **API Rate Limiting**: 100 requests/minute per client
- **Input Validation**: All endpoints validate input

### Email Security
- **Verification Tokens**: Cryptographically secure random tokens
- **Token Expiration**: Time-limited email verification links
- **Resend Integration**: Third-party email delivery service

---

## Monitoring & Metrics

### Service Health Checks
- `/health` - Gateway health endpoint
- `/agent/health` - Agent service availability
- gRPC health checks for service-to-service communication

### Metrics Collection
- Chat Service metrics (action counts by type/status)
- Request/response times
- Error rates by service
- Database query performance

### Logging
- Structured logging with context
- Error tracking and reporting
- Service interaction audit trails

---

## Error Handling & Resilience

### Error Types
```go
ErrInvalidIdempotencyKey
ErrActionNotPending
ErrActionExpired
ErrActionValidation
ErrActionConflict
```

### Resilience Patterns
1. **Idempotency**: Duplicate request detection via idempotency keys
2. **Timeouts**: Configurable request timeouts
3. **Retries**: Service-level retry logic for transient failures
4. **Graceful Degradation**: Rate limiting prevents cascade failures

---

## API Gateway Features

### Routing Rules
```
POST   /auth/*              → Auth Service
GET/PUT/DELETE /profile/*   → Auth Service (protected)
GET/POST/PUT/DELETE /calendar/* → Calendar Service
GET/POST/PUT/DELETE /location/* → Location Service
GET/POST /integration/*     → Integration Service
POST /agent/*               → Agent Service
POST /chat/*                → Chat Service
GET  /health                → Health Check
```

### Middleware Pipeline
```
Request
  ↓
Rate Limiting Middleware
  ↓
Authentication Middleware (if protected route)
  ↓
Service Router
  ↓
Service Handler
  ↓
Response
```

---

## Summary

The Orbit-core architecture provides:

✅ **Modularity**: Independent services with clear responsibilities  
✅ **Scalability**: Foundation for microservices evolution  
✅ **Maintainability**: Separation of concerns and interface-based design  
✅ **Security**: Multi-layer authentication and data protection  
✅ **Extensibility**: New services can be added to the gateway  
✅ **Reliability**: Error handling, rate limiting, and graceful degradation  
✅ **Developer Experience**: Single codebase for rapid prototyping  

Each service communicates through well-defined HTTP REST APIs or gRPC interfaces, allowing independent scaling and deployment when needed.

