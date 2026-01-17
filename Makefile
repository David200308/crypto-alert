.PHONY: build run test clean deps build-api run-api frontend-install frontend-dev frontend-build

# Build the application
build:
	@echo "Building crypto-alert..."
	@go build -o bin/crypto-alert cmd/main.go
	@echo "Build complete: bin/crypto-alert"

# Build the API server
build-api:
	@echo "Building log API server..."
	@go build -o bin/log-api cmd/api/main.go
	@echo "Build complete: bin/log-api"

# Run the application
run:
	@echo "Running crypto-alert..."
	@go run cmd/main.go

# Run the API server
run-api:
	@echo "Running log API server..."
	@go run cmd/api/main.go

# Frontend commands
frontend-install:
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install

frontend-dev:
	@echo "Starting frontend development server..."
	@cd frontend && npm run dev

frontend-build:
	@echo "Building frontend for production..."
	@cd frontend && npm run build

# Docker commands
docker-build:
	@echo "Building frontend Docker image..."
	@docker-compose build

docker-up:
	@echo "Starting frontend Docker container..."
	@docker-compose up -d

docker-down:
	@echo "Stopping frontend Docker container..."
	@docker-compose down

docker-logs:
	@echo "Showing frontend Docker logs..."
	@docker-compose logs -f frontend

docker-dev:
	@echo "Starting frontend Docker container (development mode)..."
	@docker-compose -f docker-compose.dev.yml up

docker-dev-down:
	@echo "Stopping frontend Docker container (development mode)..."
	@docker-compose -f docker-compose.dev.yml down

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run || echo "Install golangci-lint for linting: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

