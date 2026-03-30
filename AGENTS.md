# Orbit Core - Agent Development Guide

This document provides guidance for AI agents working with the Orbit Core codebase.

## Project Overview

Orbit Core is a **modular monolith** backend for the Orbit personal productivity platform, built with **Go 1.26**. It provides REST APIs for calendar management, chat, location tracking, integrations, habit tracking, and push notifications.

## Architecture

### Services

| Service | Package | Description |
|---------|---------|-------------|
| Gateway | `internal/gateway` | HTTP router, rate limiting, auth middleware |
| Auth | `internal/auth` | JWT authentication, user management |
| Calendar | `internal/calendar` | Events, tasks, recurring events |
| Location | `internal/location` | Location tracking |
| Integration | `internal/integration` | Google Calendar, import/export |
| Chat | `internal/chat` | AI chatbot, action confirmations |
| Habit | `internal/habit` | Habit detection and suggestions |
| Notification | `internal/notification` | FCM push notifications |

### Infrastructure

- **PostgreSQL** - Primary database for users, events, tasks, locations
- **MongoDB** - Chat data (conversations, messages, pending actions)
- **Redis** - Rate limiting, session caching, Asynq queue
- **gRPC** - Communication with external AI agent (orbit-orbi)

## Code Organization

```
cmd/orbit-core/main.go      # Application entry point
internal/                   # Service implementations
  auth/                     # Authentication service
  calendar/                 # Calendar/events service
  chat/                     # Chat service with MongoDB
  gateway/                  # API gateway/router
  habit/                    # Habit tracking
  integration/              # External integrations
  location/                 # Location service
  notification/             # Push notifications
  shared/                   # Shared utilities (db, models)
pkg/                        # Shared packages
  config/                   # Configuration
  fcm/                      # Firebase Cloud Messaging
  grpc/                     # gRPC client/server
  logger/                   # Structured logging
  metrics/                  # Prometheus metrics
  middleware/               # Auth, rate limiting
docs/openapi/              # OpenAPI specification
  paths/                    # API endpoint definitions
  schemas/                  # Request/response schemas
  components/               # Security schemes
migrations/                # Database migrations
proto/                     # Protobuf definitions
scripts/                   # Build/deploy scripts
```

## Key Patterns

### Repository Pattern
Each service implements repositories for data access:
```go
type Repository interface {
    Create(ctx context.Context, entity Entity) error
    GetByID(ctx context.Context, id string) (Entity, error)
    Update(ctx context.Context, entity Entity) error
    Delete(ctx context.Context, id string) error
}
```

### Service Interface Pattern
Services define interfaces for dependency injection:
```go
type AuthServiceInterface interface {
    RegisterRoutes(router *mux.Router)
    RegisterProtectedRoutes(router *mux.Router)
}
```

### Middleware Pattern
Authentication and rate limiting via HTTP middleware:
```go
authMiddleware := middleware.NewAuthMiddleware(jwtSecret)
rateLimiter := middleware.NewRateLimiter(redisAddr, 100, 1*time.Minute)
```

## Common Tasks

### Adding a New API Endpoint

1. **Define the route** in the appropriate service's `RegisterRoutes` method
2. **Create the handler** in the service file
3. **Add OpenAPI spec** in `docs/openapi/paths/<service>.yaml`
4. **Add schemas** in `docs/openapi/schemas/<service>.yaml` if needed
5. **Run** `scripts/merge-openapi.sh` to regenerate `docs/openapi.yaml`

### Adding a New Database Table

1. **Create migration** in `migrations/` using Atlas:
   ```bash
   make generate-migration name=add_new_table
   ```
2. **Define models** in `internal/shared/models/`
3. **Generate SQL code** with `sqlc generate`
4. **Create repository** in the appropriate service

### Adding a New Service

1. Create package in `internal/<service>/`
2. Implement service with `RegisterRoutes` method
3. Add to `gateway.ServiceConfig` in `internal/gateway/service.go`
4. Register routes in `setupRoutes()` method

## Configuration

Configuration is loaded from environment variables via `pkg/config/config.go`:

| Variable | Description |
|----------|-------------|
| `SERVER_PORT` | HTTP server port (default: 8080) |
| `DB_HOST`, `DB_PORT` | PostgreSQL connection |
| `MONGODB_URI` | MongoDB connection |
| `REDIS_ADDR` | Redis address |
| `JWT_SECRET` | JWT signing key |
| `GRPC_SERVER_PORT` | gRPC server port (default: 50052) |
| `ORBI_HOST`, `ORBI_PORT` | External AI agent address |

## Testing

Run tests with:
```bash
make test
```

## Database Migrations

Using Atlas for migrations:
```bash
# Generate new migration
make generate-migration name=add_users

# Apply migrations
make migrate-apply DATABASE_URL="postgres://..."
```

## Protobuf

Generate gRPC code:
```bash
make proto
```

## OpenAPI

The OpenAPI spec is generated from modular YAML files:
- `docs/openapi/paths/` - API endpoints
- `docs/openapi/schemas/` - Request/response models
- `docs/openapi/components/` - Security schemes

To regenerate after changes:
```bash
bash scripts/merge-openapi.sh
```

## gRPC Services

Orbit Core exposes two gRPC services for the external AI agent:

1. **CalendarService** - Full CRUD operations for events
2. **CalendarDataService** - Read-only access to calendar data

Proto files are in `proto/calendar/`.

## Important Files

- `cmd/orbit-core/main.go` - Application bootstrap
- `internal/gateway/service.go` - Route registration
- `pkg/config/config.go` - Configuration loading
- `pkg/middleware/auth.go` - JWT authentication
- `pkg/middleware/ratelimit.go` - Rate limiting
- `docs/openapi.yaml` - API documentation

## Dependencies

Key dependencies (see `go.mod`):
- `github.com/gorilla/mux` - HTTP router
- `github.com/jackc/pgx/v5` - PostgreSQL driver
- `github.com/redis/go-redis/v9` - Redis client
- `github.com/hibiken/asynq` - Task queue
- `github.com/golang-jwt/jwt/v5` - JWT
- `firebase.google.com/go/v4` - FCM
- `google.golang.org/grpc` - gRPC

## Docker

Development environment:
```bash
docker-compose up -d
```

Build image:
```bash
make docker-build
```
