.PHONY: build run test clean deps

# Build the application
build:
	@echo "Building crypto-alert..."
	@go build -o bin/crypto-alert cmd/main.go
	@echo "Build complete: bin/crypto-alert"

# Run the application
run:
	@echo "Running crypto-alert..."
	@go run cmd/main.go

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

