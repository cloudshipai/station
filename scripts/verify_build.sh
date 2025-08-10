#!/bin/bash

# Verify that the Station project builds successfully
set -e

echo "🔨 Building Station Project"
echo "============================"

# Set required environment variables
export ENCRYPTION_KEY=$(openssl rand -hex 32)

echo "📋 Checking Go modules..."
go mod tidy

echo "🏗️ Building main binary..."
go build -o bin/station ./cmd/main.go

echo "✅ Build completed successfully!"
echo "Binary available at: bin/station"

echo ""
echo "🔧 Quick validation..."
echo "ENCRYPTION_KEY is required to run the server."
echo "Use: ENCRYPTION_KEY=\$(openssl rand -hex 32) ./bin/station"