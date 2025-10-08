# Contributing to Orbit Core

Thank you for your interest in contributing to Orbit Core! This document provides guidelines and instructions for contributing to this project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/Orbit-core.git`
3. Create a new branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Test your changes
6. Commit your changes: `git commit -am 'Add some feature'`
7. Push to the branch: `git push origin feature/your-feature-name`
8. Submit a pull request

## Development Setup

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 15 or higher
- Redis
- Docker (optional, for containerized development)

### Installation

```bash
# Clone the repository
git clone https://github.com/waydxd/Orbit-core.git
cd Orbit-core

# Install dependencies
go mod download

# Copy environment variables
cp .env.example .env
# Edit .env with your configuration

# Run the application
go run cmd/orbit-core/main.go
```

## Code Standards

### Go Code Style

- Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines
- Use `gofmt` to format your code
- Run `go vet` to check for common mistakes
- Write meaningful commit messages

### Testing

- Write tests for new features
- Ensure all tests pass before submitting a PR
- Aim for high test coverage
- Run tests with: `go test ./...`

### Code Organization

```
- cmd/           - Application entry points
- internal/      - Private application code (services)
- pkg/           - Public libraries that can be used by external applications
- internal/shared/ - Shared internal utilities and models
```

## Pull Request Process

1. Update the README.md with details of changes if applicable
2. Update documentation for any API changes
3. Ensure all tests pass
4. Follow the existing code style
5. Provide a clear description of the changes

## Module Guidelines

When adding or modifying services:

1. **Maintain Modularity**: Each service should be independent and well-isolated
2. **Follow Interfaces**: Use interfaces for service communication
3. **Error Handling**: Always handle errors appropriately
4. **Logging**: Use the centralized logger for all logging
5. **Configuration**: Use the config package for all configuration

## Commit Message Guidelines

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

Example:
```
Add user authentication endpoint

- Implement JWT token generation
- Add Argon2id password hashing
- Update API documentation

Closes #123
```

## License

By contributing, you agree that your contributions will be licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).
