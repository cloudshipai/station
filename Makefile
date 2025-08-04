# Station Makefile
.PHONY: build clean install dev test lint kill-ports stop-station

# Build configuration
BINARY_NAME=stn
BUILD_DIR=./bin
MAIN_PACKAGE=./cmd/main

# Version information
VERSION ?= v0.1.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')

# Build flags for version info
LDFLAGS := -ldflags "-X 'station/internal/version.Version=$(VERSION)' \
                    -X 'station/internal/version.BuildTime=$(BUILD_TIME)'"

# Build the station binary
build:
	@echo "🔨 Building Station $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "✅ Built $(BUILD_DIR)/$(BINARY_NAME)"

local-install: build
	mv ./bin/stn ~/.local/bin
# Build and install to $GOPATH/bin
install:
	@echo "📦 Installing Station $(VERSION) to $$GOPATH/bin..."
	go install $(LDFLAGS) $(MAIN_PACKAGE)
	@echo "✅ Station installed! Run 'stn --help' to get started"

# Development build (faster, no optimizations)
dev:
	@echo "🔨 Building Station $(VERSION) (development)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "✅ Built ./$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	@echo "✅ Clean complete"

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "✅ Tests completed"

# Test with coverage report
test-coverage:
	@echo "🧪 Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

# Integration tests
test-integration:
	@echo "🧪 Running integration tests..."
	go test -v -race -tags=integration ./...
	@echo "✅ Integration tests completed"

# Benchmark tests
test-bench:
	@echo "🧪 Running benchmarks..."
	go test -bench=. -benchmem ./...
	@echo "✅ Benchmarks completed"

# Run linting
lint:
	@echo "🔍 Running linter..."
	golangci-lint run

# Quick setup for new users
setup:
	@echo "🚀 Setting up Station..."
	@$(MAKE) dev
	@echo "✅ Setup complete! Run './stn init' to initialize configuration"

# Kill processes on Station ports
kill-ports:
	@echo "🔪 Killing processes on Station ports..."
	-@lsof -ti:2222 | xargs -r kill -9 2>/dev/null || true
	-@lsof -ti:3000 | xargs -r kill -9 2>/dev/null || true
	-@lsof -ti:8080 | xargs -r kill -9 2>/dev/null || true
	@echo "✅ Ports cleared"

# Stop station processes
stop-station:
	@echo "🛑 Stopping Station processes..."
	-@pkill -f "./stn" || true
	-@pkill -f "stn serve" || true
	@$(MAKE) kill-ports
	@echo "✅ Station stopped"

# Show version information
version:
	@echo "Station Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"

# Show usage help
help:
	@echo "Station Build Commands:"
	@echo "  make build      - Build optimized binary to ./bin/stn"
	@echo "  make dev        - Build development binary to ./stn"  
	@echo "  make install    - Install to $$GOPATH/bin"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make test       - Run tests"
	@echo "  make lint       - Run linter"
	@echo "  make setup      - Quick setup for development"
	@echo "  make version    - Show version information"
	@echo "  make kill-ports - Kill processes on ports 2222, 3000, 8080"
	@echo "  make stop-station - Stop all Station processes and clear ports"
	@echo ""
	@echo "Version Control:"
	@echo "  make build VERSION=v1.2.3 - Build with custom version"

# Default target
all: build
