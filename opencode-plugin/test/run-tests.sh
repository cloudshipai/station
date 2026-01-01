#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building plugin..."
cd ..
bun run build
cd test

echo "Starting test infrastructure..."
docker compose up -d

echo "Waiting for services to be healthy..."
sleep 10

docker compose logs opencode

echo "Running integration tests..."
NATS_URL="nats://localhost:4222" bun run harness.ts

echo "Tests completed. Stopping infrastructure..."
docker compose down -v

echo "Done!"
