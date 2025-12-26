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

6. **Chat Service**
    - Provides a complete backend implementation for an AI-powered chatbot with action confirmation workflows.

## Core Principles

- **Modularity**: Each service is implemented as a distinct module to ensure clear separation of concerns, enabling easier refactoring into microservices when needed.
- **Scalability**: Designed to support rapid prototyping while laying the foundation for future scalability.
- **Data Integrity**: Relies on **PostgreSQL** as the primary relational database to ensure robust data consistency.

## Quick Start Guide

Get Orbit Core up and running in minutes!

### Prerequisites

Ensure you have the following installed:
- [Go 1.21+](https://golang.org/dl/)
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- [PostgreSQL 15+](https://www.postgresql.org/download/) (optional if using Docker)
- [Redis](https://redis.io/download/) (optional if using Docker)

### Option 1: Quick Start with Docker (Recommended)

#### Step 1: Clone the Repository
```bash
git clone https://github.com/waydxd/Orbit-core.git
cd Orbit-core
```

#### Step 2: Start Services
```bash
docker-compose up -d
```

This will start:
- PostgreSQL (port 5432)
- Redis (port 6379)
- Orbit Core application (port 8080)

#### Step 3: Run Database Migrations
```bash
docker-compose exec postgres psql -U postgres -d orbit -f /migrations/001_initial_schema.sql
```

#### Step 4: Verify the Application
```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "service": "gateway"
}
```

### Option 2: Manual Setup (Local Development)

#### Step 1: Clone the Repository
```bash
git clone https://github.com/waydxd/Orbit-core.git
cd Orbit-core
```

#### Step 2: Install Go Dependencies
```bash
go mod download
```

#### Step 3: Set Up Environment Variables
```bash
cp .env.example .env
```

Edit `.env` with your configuration.

#### Step 4: Start PostgreSQL and Redis
(See instructions in QUICKSTART.md if needed)

#### Step 5: Run Database Migrations
```bash
psql -U postgres -d orbit -f migrations/001_initial_schema.sql
```

#### Step 6: Build and Run the Application
```bash
go run cmd/orbit-core/main.go
```

## Architecture

### System Architecture
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
   │ Auth  │  │Calendar│ │Location│ │Integration│ │  Chat    │
   │Service│  │Service │ │Service │ │ Service  │ │ Service  │
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

## API Documentation

The full API documentation is available in `docs/openapi.yaml` and a summary is provided below.

### Base URL
`http://localhost:8080/api/v1`

### Common Endpoints

- **Authentication**: `POST /auth/register`, `POST /auth/login`
- **Calendar Events**: `GET /calendar/events`, `POST /calendar/events`
- **Chat**: `POST /chat/messages`, `GET /chat/conversations/{id}`
- **Location**: `POST /location/track`, `GET /location/history`
- **Integrations**: `POST /integration/external/connect`

## Database

The database schema is defined in `migrations/001_initial_schema.sql` and includes tables for `users`, `events`, `tasks`, `locations`, `integrations`, and `sessions`. A second migration `migrations/002_chat_and_pending_actions.sql` adds tables for `conversations`, `chat_messages`, `pending_actions`, and `agent_tool_logs`.

## Contributing

Please see the [CONTRIBUTING.md](CONTRIBUTING.md) file for details on how to contribute to this project.

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0) - see the [LICENSE](LICENSE) file for details.