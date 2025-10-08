# Orbit Core

**Orbit Core** is the central repository for the modular monolith backend of the Orbit project, built with **Golang (Go)**. It houses the core business logic services that rely on a strongly relational data model, powered by **PostgreSQL**. This repository is designed to support rapid prototyping and streamlined deployment during the early stages of development, while maintaining a modular structure to facilitate a smooth evolution into a mature microservices architecture.

## Overview

The `orbit-core` repository encapsulates the following key services, implemented as isolated modules within the monolith:

1. **Gateway Service**
   - Acts as the **single entry point** for all client requests.
   - Handles routing of traffic to internal services and enforces rate limiting using **Redis**.

2. **Authentication Service**
   - Manages **secure user verification** and session handling.
   - Implements **JWTs (JSON Web Tokens)** for authentication and uses **Argon2id** for secure credential hashing, with data stored in **PostgreSQL**.

3. **Calendar & Task Service**
   - Provides **core scheduling functionality** and event management.
   - Persists all task and event data in **PostgreSQL**.

4. **Location Service**
   - Manages location-based features, including location tracking and geolocation functionality.

5. **Integration Service**
   - Handles integration with external APIs and data synchronization.

## Core Principles

- **Modularity**: Each service is implemented as a distinct module to ensure clear separation of concerns, enabling easier refactoring into microservices when needed.
- **Scalability**: Designed to support rapid prototyping while laying the foundation for future scalability.
- **Data Integrity**: Relies on **PostgreSQL** as the primary relational database to ensure robust data consistency.

## Related Repositories

To maintain clear boundaries and accommodate different technology stacks and deployment requirements, the following components are maintained in separate repositories:

1. **AI/ML Modules (Python Backend)**
   - Contains services like the **Intelligence Service** for tasks such as natural language parsing (DistilBERT), recommendation generation (TFRS), and LLM inference (vLLM).
   - Isolated due to its **Python-based** tech stack and deployment on **GPU-accelerated nodes** (e.g., HKUST Academic Cloud).

2. **Deployment Repository**
   - Stores high-level deployment configurations, including **Kubernetes** setups and **CI/CD pipelines**.

## Project Structure

```
orbit-core/
├── cmd/
│   └── orbit-core/          # Main application entry point
├── internal/
│   ├── gateway/             # Gateway Service (routing, rate limiting)
│   ├── auth/                # Authentication Service (JWT, Argon2id)
│   ├── calendar/            # Calendar & Task Service
│   ├── location/            # Location Service
│   ├── integration/         # Integration Service
│   └── shared/
│       ├── models/          # Shared data models
│       └── database/        # Database utilities
├── pkg/
│   ├── config/              # Configuration management
│   ├── logger/              # Logging utilities
│   └── middleware/          # HTTP middleware (rate limiting, etc.)
├── go.mod
└── README.md
```

## Getting Started

### Prerequisites
- **Go**: Version 1.21 or higher
- **PostgreSQL**: Version 15 or higher
- **Redis**: For rate limiting in the Gateway Service
- **Docker**: For local development and testing

### Installation
1. Clone the repository:
   ```bash
   git clone https://github.com/waydxd/Orbit-core.git
   cd Orbit-core
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Set up environment variables (create a `.env` file):
   ```bash
   # Server Configuration
   SERVER_PORT=8080
   SERVER_HOST=0.0.0.0

   # PostgreSQL Configuration
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=postgres
   DB_NAME=orbit
   DB_SSLMODE=disable

   # Redis Configuration
   REDIS_HOST=localhost
   REDIS_PORT=6379
   REDIS_PASSWORD=
   REDIS_DB=0

   # Authentication Configuration
   JWT_SECRET=your-secret-key-change-in-production
   JWT_EXPIRATION_HOURS=24
   ```

4. Run the application:
   ```bash
   go run cmd/orbit-core/main.go
   ```

### Development

#### Build the application:
```bash
go build -o bin/orbit-core cmd/orbit-core/main.go
```

#### Run tests:
```bash
go test ./...
```

#### Run with Docker Compose (coming soon):
```bash
docker-compose up
```

## API Endpoints

### Gateway Service
- `GET /health` - Health check endpoint

### Authentication Service
- `POST /api/v1/auth/register` - Register a new user
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/verify` - Verify JWT token

### Calendar & Task Service
- `GET /api/v1/calendar/events` - List events
- `POST /api/v1/calendar/events` - Create event
- `GET /api/v1/calendar/events/{id}` - Get event
- `PUT /api/v1/calendar/events/{id}` - Update event
- `DELETE /api/v1/calendar/events/{id}` - Delete event
- `GET /api/v1/calendar/tasks` - List tasks
- `POST /api/v1/calendar/tasks` - Create task
- `GET /api/v1/calendar/tasks/{id}` - Get task
- `PUT /api/v1/calendar/tasks/{id}` - Update task
- `DELETE /api/v1/calendar/tasks/{id}` - Delete task

### Location Service
- `POST /api/v1/location/track` - Track location
- `GET /api/v1/location/history` - Get location history
- `GET /api/v1/location/current` - Get current location
- `GET /api/v1/location/nearby` - Find nearby locations

### Integration Service
- `POST /api/v1/integration/sync` - Sync data with external APIs
- `POST /api/v1/integration/webhooks` - Handle webhooks
- `POST /api/v1/integration/external/connect` - Connect external service
- `POST /api/v1/integration/external/disconnect` - Disconnect external service
- `GET /api/v1/integration/external/status` - Get integration status

## Technology Stack

- **Language**: Go 1.21+
- **Web Framework**: gorilla/mux for routing
- **Database**: PostgreSQL 15+
- **Cache/Rate Limiting**: Redis
- **Authentication**: JWT (golang-jwt), Argon2id password hashing
- **Logging**: slog (standard library)

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0) - see the [LICENSE](LICENSE) file for details.
