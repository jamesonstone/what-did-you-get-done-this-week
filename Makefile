.PHONY: help build test clean docker-build docker-up docker-down migrations cli scheduler

# Default target
help:
	@echo "Available targets:"
	@echo "  build         - Build all binaries"
	@echo "  test          - Run tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker images"
	@echo "  docker-up     - Start Docker Compose services"
	@echo "  docker-down   - Stop Docker Compose services"
	@echo "  migrations    - Run database migrations"
	@echo "  cli           - Build CLI binary"
	@echo "  scheduler     - Build scheduler binary"

# Build all binaries
build: cli scheduler

# Build CLI binary
cli:
	go build -o bin/cli ./cmd/cli

# Build scheduler binary
scheduler:
	go build -o bin/scheduler ./cmd/scheduler

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Build Docker images
docker-build:
	docker-compose build

# Start Docker Compose services
docker-up:
	docker-compose up -d

# Stop Docker Compose services
docker-down:
	docker-compose down

# Run database migrations using CLI
migrations:
	./bin/cli db migrate

# Development setup
dev-setup: docker-up migrations
	@echo "Development environment ready!"
	@echo "PostgreSQL: localhost:5432"
	@echo "MailHog UI: http://localhost:8025"
	@echo "LocalStack: http://localhost:4566"

# Create .env from example
env:
	cp .env.example .env
	@echo "Please edit .env with your configuration"