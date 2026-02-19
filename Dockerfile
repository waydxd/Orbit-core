# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o orbit-core cmd/orbit-core/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl


WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/orbit-core .

# Copy migrations
COPY migrations ./migrations

# Expose port
EXPOSE 8080

# Run the application
CMD ["./orbit-core"]
