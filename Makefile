.PHONY: all build test test-coverage bench clean fmt vet lint deps dev-deps check help release gen-grpc run-examples

# Project info
PROJECT_NAME := gomcp
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | cut -d ' ' -f 3)

# Build flags
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GoVersion=$(GO_VERSION)

# Default target
all: fmt vet lint test build

# Build the project
build:
	@echo "Building $(PROJECT_NAME)..."
	go build -v -ldflags "$(LDFLAGS)" ./...

# Build examples
build-examples:
	@echo "Building examples..."
	@for dir in examples/*/; do \
		if [ -f "$$dir/main.go" ]; then \
			echo "Building $$dir..."; \
			(cd "$$dir" && go build -v .); \
		fi \
	done

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Run specific test packages
test-server:
	@echo "Running server tests..."
	go test -v ./server/...

test-client:
	@echo "Running client tests..."
	go test -v ./client/...

test-transport:
	@echo "Running transport tests..."
	go test -v ./transport/...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	go clean ./...
	rm -f coverage.out coverage.html
	@for dir in examples/*/; do \
		if [ -f "$$dir/main.go" ]; then \
			(cd "$$dir" && go clean .); \
		fi \
	done

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install with: make dev-deps"; \
	fi

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Install development tools
dev-deps:
	@echo "Installing development dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Generate gRPC code
gen-grpc:
	@echo "Generating gRPC code from Protocol Buffer definitions..."
	@if [ -f "./transport/grpc/generate.sh" ]; then \
		./transport/grpc/generate.sh; \
	else \
		echo "gRPC generation script not found"; \
	fi

# Run example servers
run-stdio-example:
	@echo "Running stdio example..."
	@if [ -f "./examples/minimal/server/main.go" ]; then \
		cd ./examples/minimal/server && go run main.go; \
	else \
		echo "Stdio example not found"; \
	fi

run-websocket-example:
	@echo "Running websocket example..."
	@if [ -f "./examples/websocket/main.go" ]; then \
		cd ./examples/websocket && go run main.go; \
	else \
		echo "WebSocket example not found"; \
	fi

# Run all checks
check: fmt vet lint test

# Run all checks with coverage
check-coverage: fmt vet lint test-coverage

# Release workflow
release:
	@echo "Creating release..."
	@read -p "Enter release version (e.g., v1.0.0): " VERSION; \
	if [ -z "$$VERSION" ]; then \
		echo "Version cannot be empty"; \
		exit 1; \
	fi; \
	echo "Creating release $$VERSION..."; \
	git add .; \
	git commit -m "Release $$VERSION" || echo "No changes to commit"; \
	git tag "$$VERSION"; \
	git push origin "$$VERSION"; \
	git push

# Development workflow
dev: deps dev-deps fmt vet lint test
	@echo "Development environment ready!"

# CI workflow
ci: deps check-coverage
	@echo "CI checks completed!"

# Show project info
info:
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(GO_VERSION)"

# Show help
help:
	@echo "Available targets for $(PROJECT_NAME):"
	@echo ""
	@echo "Building:"
	@echo "  build          - Build the project"
	@echo "  build-examples - Build all examples"
	@echo ""
	@echo "Testing:"
	@echo "  test           - Run all tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-server    - Run server tests only"
	@echo "  test-client    - Run client tests only"
	@echo "  test-transport - Run transport tests only"
	@echo "  bench          - Run benchmarks"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt            - Format code"
	@echo "  vet            - Vet code"
	@echo "  lint           - Run linter"
	@echo "  check          - Run all checks (fmt, vet, lint, test)"
	@echo "  check-coverage - Run all checks with coverage"
	@echo ""
	@echo "Dependencies:"
	@echo "  deps           - Install dependencies"
	@echo "  dev-deps       - Install development dependencies"
	@echo ""
	@echo "Generation:"
	@echo "  gen-grpc       - Generate gRPC code"
	@echo ""
	@echo "Examples:"
	@echo "  run-stdio-example     - Run stdio example server"
	@echo "  run-websocket-example - Run websocket example server"
	@echo ""
	@echo "Utilities:"
	@echo "  clean          - Clean build artifacts"
	@echo "  dev            - Set up development environment"
	@echo "  ci             - Run CI checks"
	@echo "  release        - Create a new release"
	@echo "  info           - Show project information"
	@echo "  help           - Show this help" 