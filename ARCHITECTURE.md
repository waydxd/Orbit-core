# Architecture Overview

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Client Layer                         │
│  (Web/Mobile Apps, Third-party Services, External Systems)  │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Gateway Service (Port 8080)               │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  • Request Routing                                     │  │
│  │  • Rate Limiting (Redis)                              │  │
│  │  • API Versioning (/api/v1)                          │  │
│  │  • Health Checks                                      │  │
│  └───────────────────────────────────────────────────────┘  │
└──────┬──────────┬──────────┬──────────┬──────────┬─────────┘
       │          │          │          │          │
       ▼          ▼          ▼          ▼          ▼
   ┌───────┐  ┌────────┐ ┌────────┐ ┌─────────┐ ┌──────────┐
   │ Auth  │  │Calendar│ │Location│ │Integration│ │  (More)  │
   │Service│  │Service │ │Service │ │ Service  │ │ Services │
   └───┬───┘  └───┬────┘ └───┬────┘ └────┬─────┘ └────┬─────┘
       │          │          │           │            │
       └──────────┴──────────┴───────────┴────────────┘
                             │
                             ▼
                  ┌──────────────────────┐
                  │   Data Layer         │
                  │  ┌────────────────┐  │
                  │  │  PostgreSQL    │  │
                  │  │  (Port 5432)   │  │
                  │  └────────────────┘  │
                  │  ┌────────────────┐  │
                  │  │  Redis Cache   │  │
                  │  │  (Port 6379)   │  │
                  │  └────────────────┘  │
                  └──────────────────────┘
```

## Service Responsibilities

### 1. Gateway Service
- **Purpose**: Single entry point for all client requests
- **Technology**: gorilla/mux router
- **Features**:
  - Request routing to internal services
  - Rate limiting (100 req/min per IP using Redis)
  - CORS handling
  - API versioning
- **Endpoints**: `/health`, `/api/v1/*`

### 2. Authentication Service
- **Purpose**: User authentication and authorization
- **Technology**: JWT, Argon2id, PostgreSQL
- **Features**:
  - User registration and login
  - JWT token generation and validation
  - Argon2id password hashing (secure by design)
  - Session management
- **Endpoints**: 
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/verify`

### 3. Calendar & Task Service
- **Purpose**: Event and task management
- **Technology**: PostgreSQL
- **Features**:
  - Event CRUD operations
  - Task CRUD operations
  - Scheduling functionality
  - Time-based queries
- **Endpoints**:
  - Events: `/api/v1/calendar/events`
  - Tasks: `/api/v1/calendar/tasks`

### 4. Location Service
- **Purpose**: Location tracking and geolocation
- **Technology**: PostgreSQL with geospatial data
- **Features**:
  - Location tracking
  - Location history
  - Nearby location search
  - Geolocation queries
- **Endpoints**:
  - `POST /api/v1/location/track`
  - `GET /api/v1/location/history`
  - `GET /api/v1/location/current`
  - `GET /api/v1/location/nearby`

### 5. Integration Service
- **Purpose**: External API integration and data sync
- **Technology**: HTTP clients, webhooks
- **Features**:
  - External service connections
  - Data synchronization
  - Webhook handling
  - Integration status monitoring
- **Endpoints**:
  - `POST /api/v1/integration/sync`
  - `POST /api/v1/integration/webhooks`
  - `POST /api/v1/integration/external/connect`
  - `GET /api/v1/integration/external/status`

## Data Flow

### Authentication Flow
```
Client → Gateway → Auth Service → PostgreSQL
                        ↓
                   JWT Token
                        ↓
                     Client
```

### Event Creation Flow
```
Client → Gateway → Rate Limiter (Redis) → Calendar Service → PostgreSQL
```

### Location Tracking Flow
```
Client → Gateway → Location Service → PostgreSQL
                        ↓
                  Store coordinates
                        ↓
                Geospatial indexing
```

## Technology Stack

### Backend
- **Language**: Go 1.21+
- **Web Framework**: gorilla/mux
- **Database**: PostgreSQL 15+
- **Cache**: Redis 7+
- **Authentication**: JWT (golang-jwt/jwt)
- **Password Hashing**: Argon2id
- **Logging**: slog (standard library)

### Infrastructure
- **Containerization**: Docker
- **Orchestration**: Docker Compose (development)
- **CI/CD**: GitHub Actions (future)
- **Deployment**: Kubernetes (future)

## Development Workflow

```
Developer → Local Development → Tests → Build → Docker Image → Deploy
     ↓              ↓             ↓        ↓          ↓            ↓
  Code Edit    go run       go test   go build   docker build  docker-compose
                                                                     ↓
                                                                Production
```

## Security Measures

1. **Password Security**
   - Argon2id hashing (memory-hard algorithm)
   - Random salt per password
   - Secure storage in PostgreSQL

2. **API Security**
   - JWT-based authentication
   - Rate limiting (100 req/min)
   - HTTPS in production
   - CORS configuration

3. **Data Security**
   - Encrypted API keys for integrations
   - Cascade delete on user removal
   - Database connection pooling
   - SQL injection prevention (parameterized queries)

## Scalability Considerations

### Current State (Monolith)
- Single application deployment
- Shared PostgreSQL database
- Redis for rate limiting and caching

### Future Microservices Evolution
Each service can be extracted into:
- Independent deployment unit
- Separate database (if needed)
- Individual scaling policies
- Service-to-service communication (gRPC/HTTP)

## Module Communication

Services communicate through:
1. **Direct function calls** (current monolith)
2. **Interface-based contracts** (enables future separation)
3. **Shared data models** (in `internal/shared/models`)
4. **Common configuration** (in `pkg/config`)

This design allows for gradual migration to microservices without major refactoring.
