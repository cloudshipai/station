# Station Makefile
.PHONY: build clean install dev test lint kill-ports stop-station

# Build configuration
BINARY_NAME=station
BUILD_DIR=./bin
MAIN_PACKAGE=./cmd/main

# Build the station binary
build:
	@echo "🔨 Building Station..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "✅ Built $(BUILD_DIR)/$(BINARY_NAME)"

# Build and install to $GOPATH/bin
install:
	@echo "📦 Installing Station to $$GOPATH/bin..."
	go install $(MAIN_PACKAGE)
	@echo "✅ Station installed! Run 'station --help' to get started"

# Development build (faster, no optimizations)
dev:
	@echo "🔨 Building Station (development)..."
	go build -o $(BINARY_NAME) $(MAIN_PACKAGE)
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
	go test -v ./...

# Run linting
lint:
	@echo "🔍 Running linter..."
	golangci-lint run

# Quick setup for new users
setup:
	@echo "🚀 Setting up Station..."
	@$(MAKE) dev
	@echo "✅ Setup complete! Run './station init' to initialize configuration"

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
	-@pkill -f "./station" || true
	-@pkill -f "station serve" || true
	@$(MAKE) kill-ports
	@echo "✅ Station stopped"

# Show usage help
help:
	@echo "Station Build Commands:"
	@echo "  make build      - Build optimized binary to ./bin/station"
	@echo "  make dev        - Build development binary to ./station"  
	@echo "  make install    - Install to $$GOPATH/bin"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make test       - Run tests"
	@echo "  make lint       - Run linter"
	@echo "  make setup      - Quick setup for development"
	@echo "  make kill-ports - Kill processes on ports 2222, 3000, 8080"
	@echo "  make stop-station - Stop all Station processes and clear ports"

# Default target
all: build