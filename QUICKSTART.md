# Quick Start Guide

Get Orbit Core up and running in minutes!

## Prerequisites

Ensure you have the following installed:
- [Go 1.21+](https://golang.org/dl/)
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- [PostgreSQL 15+](https://www.postgresql.org/download/) (optional if using Docker)
- [Redis](https://redis.io/download/) (optional if using Docker)

## Option 1: Quick Start with Docker (Recommended)

### Step 1: Clone the Repository
```bash
git clone https://github.com/waydxd/Orbit-core.git
cd Orbit-core
```

### Step 2: Start Services
```bash
docker-compose up -d
```

This will start:
- PostgreSQL (port 5432)
- Redis (port 6379)
- Orbit Core application (port 8080)

### Step 3: Run Database Migrations
```bash
docker-compose exec postgres psql -U postgres -d orbit -f /migrations/001_initial_schema.sql
```

### Step 4: Verify the Application
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

### Step 5: Test the API

**Register a user:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securePassword123"
  }'
```

**Login:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securePassword123"
  }'
```

You'll receive a JWT token in the response!

### Step 6: Stop Services
```bash
docker-compose down
```

---

## Option 2: Manual Setup (Local Development)

### Step 1: Clone the Repository
```bash
git clone https://github.com/waydxd/Orbit-core.git
cd Orbit-core
```

### Step 2: Install Go Dependencies
```bash
go mod download
```

### Step 3: Set Up Environment Variables
```bash
cp .env.example .env
```

Edit `.env` with your configuration:
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
JWT_SECRET=change-this-in-production
JWT_EXPIRATION_HOURS=24
```

### Step 4: Start PostgreSQL and Redis

**Using Docker:**
```bash
# PostgreSQL
docker run -d \
  --name orbit-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=orbit \
  -p 5432:5432 \
  postgres:15-alpine

# Redis
docker run -d \
  --name orbit-redis \
  -p 6379:6379 \
  redis:7-alpine
```

**Or install locally:**
- Follow PostgreSQL installation: https://www.postgresql.org/download/
- Follow Redis installation: https://redis.io/download/

### Step 5: Run Database Migrations
```bash
psql -U postgres -d orbit -f migrations/001_initial_schema.sql
```

### Step 6: Build and Run the Application
```bash
# Build
go build -o bin/orbit-core cmd/orbit-core/main.go

# Run
./bin/orbit-core
```

Or run directly:
```bash
go run cmd/orbit-core/main.go
```

### Step 7: Verify the Application
```bash
curl http://localhost:8080/health
```

---

## Using the Makefile

The project includes a Makefile for common tasks:

```bash
# Show all available commands
make help

# Build the application
make build

# Run the application
make run

# Run tests
make test

# Clean build artifacts
make clean

# Build Docker image
make docker-build

# Start services with Docker Compose
make docker-up

# Stop services
make docker-down

# View logs
make docker-logs

# Format code
make fmt

# Tidy dependencies
make tidy
```

---

## Testing the API

### Health Check
```bash
curl http://localhost:8080/health
```

### Authentication

**Register:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

**Login:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

Save the JWT token from the response for authenticated requests.

### Calendar Events

**Create Event:**
```bash
curl -X POST http://localhost:8080/api/v1/calendar/events \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "title": "Team Meeting",
    "description": "Weekly sync",
    "start_time": "2024-01-15T10:00:00Z",
    "end_time": "2024-01-15T11:00:00Z",
    "location": "Room A"
  }'
```

**List Events:**
```bash
curl http://localhost:8080/api/v1/calendar/events
```

### Location Tracking

**Track Location:**
```bash
curl -X POST http://localhost:8080/api/v1/location/track \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "latitude": 22.3193,
    "longitude": 114.1694,
    "address": "Hong Kong University of Science and Technology"
  }'
```

**Get Location History:**
```bash
curl "http://localhost:8080/api/v1/location/history?user_id=user-123"
```

---

## Troubleshooting

### Port Already in Use
If port 8080 is already in use, change it in `.env`:
```bash
SERVER_PORT=9000
```

### Database Connection Failed
Ensure PostgreSQL is running:
```bash
# Check if PostgreSQL is running
docker ps | grep postgres

# Or for local installation
pg_isready
```

### Redis Connection Failed
Ensure Redis is running:
```bash
# Check if Redis is running
docker ps | grep redis

# Or for local installation
redis-cli ping
```

### Build Errors
Update dependencies:
```bash
go mod tidy
go mod download
```

---

## Next Steps

1. **Read the [API Documentation](API.md)** for detailed endpoint information
2. **Check the [Architecture Guide](ARCHITECTURE.md)** to understand the system design
3. **Review the [Database Schema](DATABASE.md)** for data model details
4. **Read [CONTRIBUTING.md](CONTRIBUTING.md)** if you want to contribute

---

## Production Deployment

For production deployment, ensure:

1. **Change JWT Secret**: Update `JWT_SECRET` in environment variables
2. **Enable HTTPS**: Use a reverse proxy (nginx/traefik)
3. **Secure Database**: Use strong passwords and enable SSL
4. **Configure Redis Password**: Set `REDIS_PASSWORD`
5. **Use Environment-Specific Configs**: Separate dev/staging/prod configs
6. **Enable Logging**: Configure log aggregation (ELK, Datadog, etc.)
7. **Set Up Monitoring**: Use Prometheus, Grafana for metrics

For Kubernetes deployment, see the deployment repository (coming soon).
