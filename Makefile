.PHONY: all test lint clean bench coverage deps fmt vet check tag-release

# Project info
PROJECT_NAME := gomcp
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | cut -d ' ' -f 3)

# Default target
all: test lint

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	golangci-lint run

# Clean build artifacts
clean:
	go clean
	rm -rf dist/
	rm -f coverage.out coverage.html

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Generate coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Run all checks
check: fmt vet lint test

# Create a git tag for release (SDK release process)
tag-release:
	@if [ -z "$(VERSION_TAG)" ]; then echo "VERSION_TAG is required. Usage: make tag-release VERSION_TAG=v1.5.0"; exit 1; fi
	@echo "Creating release tag $(VERSION_TAG)..."
	@git tag $(VERSION_TAG)
	@echo "Tag $(VERSION_TAG) created. Push with: git push origin $(VERSION_TAG)"
	@echo "Don't forget to also push main: git push origin main"

# Show project info
info:
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(GO_VERSION)"

# Show help
help:
	@echo "Available targets for $(PROJECT_NAME) SDK:"
	@echo ""
	@echo "Testing:"
	@echo "  test           - Run all tests"
	@echo "  coverage       - Generate coverage report"
	@echo "  bench          - Run benchmarks"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt            - Format code"
	@echo "  vet            - Vet code"
	@echo "  lint           - Run linter"
	@echo "  check          - Run all checks (fmt, vet, lint, test)"
	@echo ""
	@echo "Dependencies:"
	@echo "  deps           - Install dependencies"
	@echo ""
	@echo "Release:"
	@echo "  tag-release    - Create a git tag for release (use VERSION_TAG=v1.5.0)"
	@echo ""
	@echo "Utilities:"
	@echo "  clean          - Clean build artifacts"
	@echo "  info           - Show project information"
	@echo "  help           - Show this help" 