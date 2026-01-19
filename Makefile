.PHONY: build run test clean docker-build docker-up docker-down help install-protoc install-protoc-gen proto generate clean-proto

# Variables
BINARY_NAME=orbit-core
BINARY_PATH=bin/$(BINARY_NAME)
DOCKER_IMAGE=orbit-core:latest

# Protobuf / protoc generation settings
PROTOC_GEN_GO := google.golang.org/protobuf/cmd/protoc-gen-go@latest
PROTOC_GEN_GO_GRPC := google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
PROTO_DIRS := proto
PROTO_FILES := $(shell find $(PROTO_DIRS) -name '*.proto')
GOBIN ?= $(shell go env GOPATH)/bin

install-tools: ## Install development tools (sqlc, atlas)
	@echo "Installing tools..."
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@go install ariga.io/atlas/cmd/atlas@latest
	@echo "Tools installed"

generate: install-tools ## Generate code
	@echo "Generating SQL code..."
	@sqlc generate
	@echo "SQL code generated"

generate-migration: ## Generate a new migration file (usage: make generate-migration name=add_users)
	@echo "Generating migration: $(name)"
	@atlas migrate diff $(name) \
		--env local

migrate-apply: ## Apply migrations
	@echo "Applying migrations..."
	@atlas migrate apply \
		--env local \
		--url "$(DATABASE_URL)"

help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_PATH) cmd/orbit-core/main.go
	@echo "Build complete: $(BINARY_PATH)"

run: ## Run the application
	@echo "Running $(BINARY_NAME)..."
	@go run cmd/orbit-core/main.go

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@echo "Clean complete"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

docker-up: ## Start services with Docker Compose
	@echo "Starting services with Docker Compose..."
	@docker-compose up -d
	@echo "Services started"

docker-down: ## Stop services with Docker Compose
	@echo "Stopping services with Docker Compose..."
	@docker-compose down
	@echo "Services stopped"

docker-logs: ## View Docker Compose logs
	@docker-compose logs -f

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@echo "Dependencies downloaded"

tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	@go mod tidy
	@echo "Dependencies tidied"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Code formatted"

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run ./...
	@echo "Linting complete"

install-protoc: ## Install protoc (macOS/Homebrew) - or install manually on Linux/Windows
	@echo "Installing protoc (macOS/Homebrew)..."
	@if command -v brew >/dev/null 2>&1; then \
		brew install protobuf; \
	else \
		echo "brew not found: please install protoc manually: https://github.com/protocolbuffers/protobuf/releases"; exit 1; \
	fi

install-protoc-gen: ## Install Go protoc plugins (protoc-gen-go, protoc-gen-go-grpc)
	@echo "Installing protoc-gen-go and protoc-gen-go-grpc to $(GOBIN)"
	@export PATH=$$PATH:$(GOBIN):$(HOME)/go/bin; \
	go install $(PROTOC_GEN_GO); \
	go install $(PROTOC_GEN_GO_GRPC);

proto:
	@echo "Generating protobuf code..."
	@export PATH=$$PATH:$$(go env GOPATH)/bin && \
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/calendar/calendar.proto proto/calendar/agent.proto
	@echo "Protobuf code generated successfully"

generate: proto ## Alias for proto (generate protobuf Go code)

clean-proto: ## Remove generated proto Go files
	@echo "Removing generated proto Go files..."
	@find . -type f \( -name '*_pb.go' -o -name '*_grpc.pb.go' \) -print -exec rm -f {} +

all: clean build ## Clean and build
	@echo "Build pipeline complete"
