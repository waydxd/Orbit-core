# Orbit Core

**Orbit Core** is the central repository for the modular monolith backend of the Orbit project, built with **Golang (Go)**. It houses the core business logic services that rely on a strongly relational data model, powered by **PostgreSQL**. This repository is designed to support rapid prototyping and streamlined deployment during the early stages of development, while maintaining a modular structure to facilitate a smooth evolution into a mature microservices architecture.

## Tech Stack

- **Language**: Go 1.26
- **HTTP Router**: Gorilla Mux
- **Database**: PostgreSQL (primary), MongoDB (chat data)
- **Cache/Queue**: Redis
- **Task Queue**: Asynq
- **Authentication**: JWT + Argon2id password hashing
- **gRPC**: For inter-service communication with AI agent
- **Push Notifications**: Firebase Cloud Messaging (FCM)
- **Email**: Resend
- **External Integrations**: Google Calendar API

## Core Services

The `orbit-core` repository encapsulates the following key services, implemented as isolated modules within the monolith:

1. **Gateway Service**
   - Acts as the **single entry point** for all client requests
   - Handles routing of traffic to internal services
   - Enforces rate limiting using **Redis** (100 requests/minute per client)
   - API versioning at `/api/v1`

2. **Authentication Service**
   - Manages **secure user verification** and session handling
   - Implements **JWTs (JSON Web Tokens)** for authentication
   - Uses **Argon2id** for secure credential hashing
   - Email verification and password reset flows via **Resend**
   - User profile management

3. **Calendar & Task Service**
   - Provides **core scheduling functionality** and event management
   - Supports recurring events (daily, weekly, monthly, yearly)
   - Task management (CRUD operations)
   - Exposes gRPC services for external agent integration

4. **Location Service**
   - Manages location-based features, including location tracking and geolocation functionality

5. **Integration Service**
   - Handles integration with external APIs (Google Calendar)
   - Supports import/export in multiple formats (ICS, CSV)
   - OAuth 2.0 flow for Google Calendar

6. **Chat Service**
   - Provides a complete backend implementation for an AI-powered chatbot with action confirmation workflows
   - Multi-turn conversation management with MongoDB persistence
   - Pending action tracking with confirmation workflows
   - Idempotency support via idempotency keys

7. **Habit Tracking Service**
   - Automatically detects recurring event patterns based on user behavior
   - When an event (with the same title, time, duration, and day of week) occurs 3+ times, suggests making it a recurring habit
   - Allows users to accept suggestions to auto-schedule the event for the next 5 years

8. **Notification Service**
   - Push notifications via **Firebase Cloud Messaging (FCM)**
   - Event reminder scheduling using **Asynq** task queue
   - Device token management (iOS/Android)

## Architecture

### System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Gateway Service (Port 8080)               │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  • Request Routing                                     │  │
│  │  • Rate Limiting (Redis)                              │  │
│  │  • API Versioning (/api/v1)                          │  │
│  │  • Health Checks                                      │  │
│  └───────────────────────────────────────────────────────┘  │
└──────┬──────────┬──────────┬──────────┬──────────┬──────────┬──────┐
       │          │          │          │          │          │      │
       ▼          ▼          ▼          ▼          ▼          ▼      ▼
   ┌───────┐  ┌────────┐ ┌────────┐ ┌─────────┐ ┌──────────┐ ┌──────┐ ┌───────┐
   │ Auth  │  │Calendar│ │Location│ │Integration│ │  Chat   │ │Habit │ │Notif  │
   │Service│  │Service │ │Service │ │ Service  │ │ Service │ │Service│ │Service│
   └───┬───┘  └───┬────┘ └───┬────┘ └────┬─────┘ └────┬─────┘ └──┬───┘ └───┬───┘
       │          │          │           │            │          │         │
       └──────────┴──────────┴───────────┴────────────┴──────────┴─────────┘
                                 │
                                 ▼
                     ┌──────────────────────┐
                     │   Data Layer         │
                     │  ┌────────────────┐  │
                     │  │  PostgreSQL    │  │
                     │  │  (Port 5432)   │  │
                     │  └────────────────┘  │
                     │  ┌────────────────┐  │
                     │  │  MongoDB       │  │
                     │  │  (Chat Data)   │  │
                     │  └────────────────┘  │
                     │  ┌────────────────┐  │
                     │  │  Redis         │  │
                     │  │  (Cache/Queue) │  │
                     │  └────────────────┘  │
                     └──────────────────────┘
```

### gRPC Architecture

The system uses gRPC for communication between `orbit-core` and the external `orbit-orbi` AI agent.

- **`orbit-core`** implements and exposes:
  - `CalendarService` (CRUD operations)
  - `CalendarDataService` (read-only data access)
- **`orbit-core`** calls:
  - `AgentService` on `orbit-orbi` (for AI message processing)
- **`orbit-orbi`** (external service) implements and exposes:
  - `AgentService` (AI message processing)
- **`orbit-orbi`** (external service) calls:
  - `CalendarService` on `orbit-core` (to create/modify events)
  - `CalendarDataService` on `orbit-core` (to read calendar data)

## Quick Start Guide

### Prerequisites

Ensure you have the following installed:
- [Go 1.26+](https://golang.org/dl/)
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)

### Quick Start with Docker (Recommended)

#### Step 1: Clone the Repository
```bash
git clone https://github.com/waydxd/Orbit-core.git
cd Orbit-core
```

#### Step 2: Configure Environment
```bash
cp .env.example .env
```

Edit `.env` with your configuration (or use defaults for local development).

#### Step 3: Start Services
```bash
docker-compose up -d
```

This will start:
- PostgreSQL (port 5432)
- Redis (port 6379)
- MongoDB (port 27017)
- Orbit Orbi AI agent (port 50042)
- Orbit Core application (port 8080, gRPC 50052)
- Atlas (database migrations)

#### Step 4: Verify the Application
```bash
curl http://localhost:8080/health
```

Expected response:
```json
{"status":"healthy","service":"gateway"}
```

### Local Development

#### Step 1: Clone the Repository
```bash
git clone https://github.com/waydxd/Orbit-core.git
cd Orbit-core
```

#### Step 2: Install Dependencies
```bash
go mod download
```

#### Step 3: Set Up Environment Variables
```bash
cp .env.example .env
```

#### Step 4: Run Database Migrations
```bash
# Using Atlas (from Makefile)
make migrate-apply DATABASE_URL="postgres://postgres@localhost:5432/orbit?sslmode=disable"
```

#### Step 5: Run the Application
```bash
go run cmd/orbit-core/main.go
```

## API Documentation

The full API documentation is available in `docs/openapi.yaml`.

### Base URL
`http://localhost:8080/api/v1`

### Core Endpoints

#### Authentication
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login
- `POST /api/v1/auth/logout` - Logout
- `POST /api/v1/auth/verify` - Verify JWT token
- `POST /api/v1/auth/password-reset-request` - Request password reset
- `POST /api/v1/auth/password-reset-confirm` - Confirm password reset
- `GET /api/v1/auth/verify-email` - Verify email
- `GET /api/v1/profile` - Get user profile (protected)
- `PUT /api/v1/profile` - Update user profile (protected)

#### Calendar Events
- `GET /api/v1/calendar/events` - List events
- `POST /api/v1/calendar/events` - Create event
- `GET /api/v1/calendar/events/{id}` - Get event
- `PUT /api/v1/calendar/events/{id}` - Update event
- `DELETE /api/v1/calendar/events/{id}` - Delete event

#### Calendar Tasks
- `GET /api/v1/calendar/tasks` - List tasks
- `POST /api/v1/calendar/tasks` - Create task
- `GET /api/v1/calendar/tasks/{id}` - Get task
- `PUT /api/v1/calendar/tasks/{id}` - Update task
- `DELETE /api/v1/calendar/tasks/{id}` - Delete task

#### Recurring Events
- `GET /api/v1/calendar/recurring` - List recurring events
- `POST /api/v1/calendar/recurring/{id}/deactivate` - Deactivate recurring event

#### Chat
- `GET /api/v1/chat/conversations` - List conversations
- `GET /api/v1/chat/conversations/{id}` - Get conversation
- `POST /api/v1/chat/messages` - Send message
- `POST /api/v1/chat/actions/{id}/confirm` - Confirm action
- `POST /api/v1/chat/actions/{id}/cancel` - Cancel action
- `GET /api/v1/chat/actions/{id}` - Get action details
- `GET /api/v1/chat/metrics` - Get service metrics

#### Location
- `POST /api/v1/location/track` - Record location
- `GET /api/v1/location/history` - Get location history
- `GET /api/v1/location/current` - Get current location
- `GET /api/v1/location/nearby` - Find nearby locations

#### Integrations
- `GET /api/v1/integration/google/auth` - Initiate Google OAuth
- `GET /api/v1/integration/google/callback` - OAuth callback
- `POST /api/v1/integration/google/sync` - Sync with Google Calendar
- `POST /api/v1/integration/google/disconnect` - Disconnect Google
- `GET /api/v1/integration/google/status` - Get Google integration status
- `POST /api/v1/integration/import` - Import calendar data
- `POST /api/v1/integration/export` - Export calendar data
- `GET /api/v1/integration/external/connect` - Connect external service
- `GET /api/v1/integration/external/disconnect` - Disconnect external service
- `GET /api/v1/integration/external/status` - Get external service status

#### Habit Tracking
- `GET /api/v1/habit/suggestions` - Get habit suggestions
- `POST /api/v1/habit/suggestions/{id}/accept` - Accept habit suggestion
- `POST /api/v1/habit/suggestions/{id}/reject` - Reject habit suggestion

#### Notifications
- `POST /api/v1/fcm/token` - Register device token
- `DELETE /api/v1/fcm/token` - Delete device token
- `POST /api/v1/events/{id}/notify` - Subscribe to event notifications
- `DELETE /api/v1/events/{id}/notify` - Unsubscribe from event notifications

## Database

The database schema is managed via **Atlas** migrations in the `migrations/` directory:

- `001_initial_schema.sql` - Core tables (users, events, tasks, locations, integrations)
- `002_add_email_verified_to_users.sql` - Email verification
- `003_add_recurrence_to_events.sql` - Recurring events support
- `004_add_index_on_events_is_recurring.sql` - Performance indexing
- `005_add_user_profile_fields.sql` - Extended user profiles
- `006_habit_tracking.sql` - Habit tracking feature
- `007_add_hashtag_to_events_tasks.sql` - Hashtag support
- `008_fcm_notifications.sql` - Push notifications
- `009_event_subscriptions_asynq.sql` - Asynq task queue for notifications

## Makefile Commands

- `make build` - Build the application
- `make run` - Run the application
- `make test` - Run tests
- `make docker-up` - Start services with Docker Compose
- `make docker-down` - Stop services
- `make generate` - Generate SQL code (sqlc)
- `make generate-migration name=<name>` - Generate new migration
- `make lint` - Run linter
- `make proto` - Generate protobuf code

## Contributing

Please see the [CONTRIBUTING.md](CONTRIBUTING.md) file for details on how to contribute to this project.

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0) - see the [LICENSE](LICENSE) file for details.
