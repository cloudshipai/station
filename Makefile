# Station Makefile
.PHONY: build clean install dev test test-bundle test-bundle-watch lint kill-ports stop-station dev-ui build-ui install-ui build-with-ui local-install-ui tag-check release jaeger jaeger-down proto-gen proto-clean

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

# Build with UI embedded
build-with-ui: build-ui
	@echo "🔨 Building Station $(VERSION) with embedded UI..."
	@mkdir -p $(BUILD_DIR)
	@mkdir -p internal/ui/static
	@cp -r ui/dist/* internal/ui/static/ 2>/dev/null || true
	go build $(LDFLAGS) -tags ui -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "✅ Built $(BUILD_DIR)/$(BINARY_NAME) with embedded UI"

local-install: build
	mv ./bin/stn ~/.local/bin

# Build and install with UI embedded
local-install-ui: build-with-ui
	mv ./bin/stn ~/.local/bin
	@echo "✅ Installed Station with embedded UI to ~/.local/bin"

# Release targets
tag-check:
	@echo "🏷️ Current tags:"
	@git tag | tail -10
	@echo ""
	@echo "Next tag should be: $(shell git tag | tail -1 | awk -F. '{print $$1"."$$2"."$$3+1}')"

release: build-with-ui
	@echo "🚀 Creating release build..."
	@echo "✅ Station built with embedded UI at ./bin/stn"
	@echo "📋 To create a new tag:"
	@echo "   1. Check current tags with: make tag-check"  
	@echo "   2. Create tag: git tag v0.8.7 (or next version)"
	@echo "   3. Push tag: git push origin v0.8.7"
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

# Lighthouse-specific test targets
test-lighthouse-unit:
	@echo "🧪 Running Lighthouse unit tests..."
	go test -v ./internal/lighthouse -run TestLighthouseUnit
	@echo "✅ Lighthouse unit tests completed"

test-lighthouse-integration:
	@echo "🌐 Running Lighthouse integration tests..."
	@echo "   Endpoint: $${LIGHTHOUSE_TEST_ENDPOINT:-localhost:50051}"
	@echo "   Registration Key: $${LIGHTHOUSE_TEST_KEY:-[using default test key]}"
	go test -v ./internal/lighthouse -run TestLighthouseIntegration
	@echo "✅ Lighthouse integration tests completed"

test-lighthouse:
	@echo "🔬 Running all Lighthouse tests..."
	go test -v ./internal/lighthouse
	@echo "✅ All Lighthouse tests completed"

bench-lighthouse:
	@echo "⚡ Running Lighthouse benchmarks..."
	go test -bench=. -benchmem ./internal/lighthouse
	@echo "✅ Lighthouse benchmarks completed"

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

# UI Development workflow
dev-ui:
	@echo "🚀 Starting UI development server..."
	@cd ui && npm run dev

# Build UI for production
build-ui:
	@echo "📦 Building UI for production..."
	@cd ui && npm run build
	@echo "✅ UI built to ui/dist/"

# Install UI dependencies
install-ui:
	@echo "📦 Installing UI dependencies..."
	@cd ui && npm install
	@echo "✅ UI dependencies installed"
	@echo "Build Time: $(BUILD_TIME)"

# Bundle system development targets
test-bundle:
	@echo "🧪 Running bundle system tests..."
	@go test -v ./pkg/bundle/... -cover

test-bundle-watch:
	@echo "👀 Starting bundle test watcher (Ctrl+C to stop)..."
	@while true; do \
		go test -v ./pkg/bundle/... -cover; \
		echo ""; \
		echo "⏰ Waiting for changes... Press Ctrl+C to stop"; \
		inotifywait -r -e modify,create,delete ./pkg/bundle/ 2>/dev/null || sleep 2; \
		clear; \
	done

# Agent Bundle system development targets
test-agent-bundle:
	@echo "🤖 Running agent bundle system tests..."
	@go test -v ./pkg/agent-bundle/... -cover

test-agent-bundle-watch:
	@echo "👀 Starting agent bundle test watcher (Ctrl+C to stop)..."
	@while true; do \
		go test -v ./pkg/agent-bundle/... -cover; \
		echo ""; \
		echo "⏰ Waiting for changes... Press Ctrl+C to stop"; \
		inotifywait -r -e modify,create,delete ./pkg/agent-bundle/ 2>/dev/null || sleep 2; \
		clear; \
	done

# Combined bundle testing (both template and agent bundles)
test-bundles:
	@echo "📦 Running all bundle system tests..."
	@go test -v ./pkg/bundle/... ./pkg/agent-bundle/... -cover

# Show usage help
help:
	@echo "Station Build Commands:"
	@echo "  make build      - Build optimized binary to ./bin/stn"
	@echo "  make dev        - Build development binary to ./stn"  
	@echo "  make install    - Install to $$GOPATH/bin"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make test       - Run all tests"
	@echo "  make test-lighthouse - Run all Lighthouse tests"
	@echo "  make test-lighthouse-unit - Run Lighthouse unit tests only"
	@echo "  make test-lighthouse-integration - Run Lighthouse integration tests"
	@echo "  make lint       - Run linter"
	@echo "  make setup      - Quick setup for development"
	@echo "  make version    - Show version information"
	@echo "  make kill-ports - Kill processes on ports 2222, 3000, 8080"
	@echo "  make stop-station - Stop all Station processes and clear ports"
	@echo ""
	@echo "Bundle System Development:"
	@echo "  make test-bundle              - Run template bundle system tests"
	@echo "  make test-bundle-watch        - Watch template bundle tests"
	@echo "  make test-agent-bundle        - Run agent bundle system tests"
	@echo "  make test-agent-bundle-watch  - Watch agent bundle tests"
	@echo "  make test-bundles             - Run all bundle system tests"
	@echo ""
	@echo ""
	@echo "OpenTelemetry & Observability:"
	@echo "  make jaeger      - Start Jaeger with OTLP support for distributed tracing"
	@echo "  make jaeger-down - Stop and remove Jaeger container"
	@echo ""
	@echo "CloudShip Integration:"
	@echo "  make proto-gen    - Generate Go code from existing proto files"
	@echo "  make proto-clean  - Clean generated proto files"  
	@echo "  make proto-update - Update proto from CloudShip team and regenerate"
	@echo ""
	@echo "Version Control:"
	@echo "  make build VERSION=v1.2.3 - Build with custom version"

# Default target
all: build

# OpenTelemetry & Observability targets

# Start Jaeger with OTLP support for distributed tracing
jaeger:
	@echo "🔍 Starting Jaeger with OTLP support..."
	@docker run -d --name station-jaeger \
		-e COLLECTOR_OTLP_ENABLED=true \
		-p 16686:16686 \
		-p 4317:4317 \
		-p 4318:4318 \
		jaegertracing/all-in-one:latest
	@echo "✅ Jaeger started successfully!"
	@echo "📊 Jaeger UI: http://localhost:16686"
	@echo "🔌 OTLP gRPC: localhost:4317"
	@echo "🔌 OTLP HTTP: localhost:4318"
	@echo ""
	@echo "💡 To use with Station, set these environment variables:"
	@echo "   export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317"
	@echo "   export OTEL_EXPORTER_OTLP_PROTOCOL=grpc"
	@echo "   export OTEL_SERVICE_NAME=station"
	@echo "   export OTEL_SERVICE_VERSION=0.2.7"

# Stop and remove Jaeger container
jaeger-down:
	@echo "🛑 Stopping Jaeger..."
	@docker stop station-jaeger || true
	@docker rm station-jaeger || true
	@echo "✅ Jaeger stopped and removed"

# Proto Generation & CloudShip Integration
# These commands handle updating proto files and generating Go code for Lighthouse integration

proto-clean:
	@echo "🧹 Cleaning generated proto files..."
	@rm -f internal/lighthouse/proto/*.pb.go
	@echo "✅ Proto files cleaned"

# Generate Go code from proto files
proto-gen:
	@echo "🔄 Generating Go code from proto files..."
	@if ! which protoc >/dev/null 2>&1; then \
		echo "❌ protoc not found. Install with: brew install protobuf (macOS) or apt-get install protobuf-compiler (Ubuntu)"; \
		exit 1; \
	fi
	@if ! which protoc-gen-go >/dev/null 2>&1; then \
		echo "📦 Installing protoc-gen-go..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; \
	fi
	@if ! which protoc-gen-go-grpc >/dev/null 2>&1; then \
		echo "📦 Installing protoc-gen-go-grpc..."; \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest; \
	fi
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/lighthouse/proto/lighthouse.proto
	@echo "✅ Proto code generated successfully"

# Update proto file from CloudShip team and regenerate
proto-update:
	@echo "🔄 Updating proto files from CloudShip..."
	@if [ ! -f "$(HOME)/projects/cloudshipai/lighthouse/internal/proto/lighthouse.proto" ]; then \
		echo "❌ CloudShip proto file not found at $(HOME)/projects/cloudshipai/lighthouse/internal/proto/lighthouse.proto"; \
		echo "💡 Clone CloudShip repo or update the path in this Makefile"; \
		exit 1; \
	fi
	@cp "$(HOME)/projects/cloudshipai/lighthouse/internal/proto/lighthouse.proto" internal/lighthouse/proto/
	@$(MAKE) proto-gen
	@echo "✅ Proto files updated from CloudShip and regenerated"

