# Makefile for gorsudp
.PHONY: build test clean run install help generate-test-data

# Variables
BINARY_NAME=gorsudp
MAIN_PACKAGE=./cmd/gorsudp
BUILD_DIR=./build
VERSION=$(shell git describe --tags --always --dirty)
COMMIT=$(shell git rev-parse --short HEAD)
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S%z)

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Default target
all: build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)
	@echo "Multi-platform build complete"

# Run tests
test:
	@echo "Running tests..."
	go test -race -cover ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	go test -race -cover -v ./...

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Generate test coverage report
coverage:
	@echo "Generating coverage report..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run the application
run:
	go run $(MAIN_PACKAGE)

# Run with debug logging
run-debug:
	go run $(MAIN_PACKAGE) -debug

# Generate default configuration
generate-config:
	go run $(MAIN_PACKAGE) -generate-config

# Install the application
install:
	go install $(LDFLAGS) $(MAIN_PACKAGE)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Update dependencies
deps:
	@echo "Updating dependencies..."
	go mod tidy
	go mod download

# Generate test data
generate-test-data:
	@echo "Generating test data..."
	go run ./cmd/gorsudp-test -generate-test-data

# Run integration tests
integration-test:
	@echo "Running integration tests..."
	go test -tags=integration ./...

# Performance test
perf-test:
	@echo "Running performance tests..."
	go test -tags=perf -timeout=30m ./...

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t gorsudp:$(VERSION) -f deploy/docker/Dockerfile .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run --rm -p 8888:8888/udp -p 8080:8080 gorsudp:$(VERSION)

# Show help
help:
	@echo "Available targets:"
	@echo "  build            - Build the application"
	@echo "  build-all        - Build for multiple platforms"
	@echo "  test             - Run tests"
	@echo "  test-verbose     - Run tests with verbose output"
	@echo "  benchmark        - Run benchmarks"
	@echo "  coverage         - Generate test coverage report"
	@echo "  run              - Run the application"
	@echo "  run-debug        - Run with debug logging"
	@echo "  generate-config  - Generate default configuration"
	@echo "  install          - Install the application"
	@echo "  clean            - Clean build artifacts"
	@echo "  fmt              - Format code"
	@echo "  lint             - Run linter"
	@echo "  deps             - Update dependencies"
	@echo "  generate-test-data - Generate test data"
	@echo "  integration-test - Run integration tests"
	@echo "  perf-test        - Run performance tests"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-run       - Run Docker container"
	@echo "  help             - Show this help message"