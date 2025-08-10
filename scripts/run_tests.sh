#!/bin/bash

# Run all tests for the Station project
set -e

echo "🧪 Running Station Project Tests"
echo "================================"

# Set required environment variables for tests
export ENCRYPTION_KEY=$(openssl rand -hex 32)

echo "📦 Testing crypto package..."
go test ./pkg/crypto/ -v -count=1

echo ""
echo "⚙️ Testing config package..."
go test ./internal/config/ -v -count=1

echo ""
echo "🗄️ Testing database package..."
go test ./internal/db/ -v -count=1

echo ""
echo "📊 Testing repository layer..."
go test ./internal/db/repositories/ -v -count=1

echo ""
echo "🛠️ Testing services..."
go test ./internal/services/ -v -count=1

echo ""
echo "✅ All tests completed successfully!"

# Optional: Run with coverage
if [ "$1" = "--coverage" ]; then
    echo ""
    echo "📈 Running tests with coverage..."
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    echo "Coverage report generated: coverage.html"
fi