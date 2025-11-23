#!/bin/sh
set -e

# Production entrypoint with Litestream for GitOps deployments
echo "🚀 Starting Station with Litestream state persistence..."

# Environment validation
if [ -z "$LITESTREAM_S3_BUCKET" ] && [ -z "$LITESTREAM_ABS_BUCKET" ] && [ -z "$LITESTREAM_GCS_BUCKET" ]; then
    echo "⚠️  No Litestream replica configured. Running in ephemeral mode."
    echo "   Set LITESTREAM_S3_BUCKET, LITESTREAM_ABS_BUCKET, or LITESTREAM_GCS_BUCKET for persistence."
fi

# Fix volume permissions if running as root
if [ "$(id -u)" = "0" ]; then
    echo "🔧 Running as root - fixing volume permissions..."
    mkdir -p /data /backup /config
    chown -R station:station /data /backup /config 2>/dev/null || true
    
    # Re-execute this script as station user
    echo "🔄 Switching to station user..."
    exec su-exec station "$0" "$@"
fi

# Create data directory if it doesn't exist (as station user)
mkdir -p /data

# Restore database from replica if it exists and local DB is missing
if [ ! -f "/data/station.db" ]; then
    echo "📦 Attempting to restore database from replica..."
    if litestream restore -config /config/litestream.yml /data/station.db; then
        echo "✅ Database restored successfully from replica"
    else
        echo "💡 No existing replica found. Starting with fresh database."
        # Initialize empty database - Station will create schema on startup
        touch /data/station.db
    fi
fi

# Start Litestream replication in background
if [ -n "$LITESTREAM_S3_BUCKET" ] || [ -n "$LITESTREAM_ABS_BUCKET" ] || [ -n "$LITESTREAM_GCS_BUCKET" ]; then
    echo "🔄 Starting Litestream replication..."
    litestream replicate -config /config/litestream.yml &
    LITESTREAM_PID=$!
    
    # Wait for Litestream to initialize
    sleep 2
    echo "✅ Litestream replication active (PID: $LITESTREAM_PID)"
else
    echo "⚠️  Litestream replication disabled - no replica configuration"
fi

# Function to handle graceful shutdown
cleanup() {
    echo "🛑 Shutting down Station..."
    if [ -n "$STATION_PID" ]; then
        kill -TERM "$STATION_PID" 2>/dev/null || true
        wait "$STATION_PID" 2>/dev/null || true
    fi
    
    if [ -n "$LITESTREAM_PID" ]; then
        echo "🔄 Stopping Litestream replication..."
        kill -TERM "$LITESTREAM_PID" 2>/dev/null || true
        wait "$LITESTREAM_PID" 2>/dev/null || true
        echo "✅ Litestream stopped"
    fi
    
    echo "👋 Station shutdown complete"
    exit 0
}

# Set up signal handlers
trap cleanup SIGTERM SIGINT

# Start Station with production database path
echo "🎯 Starting Station server..."
export DATABASE_PATH="/data/station.db"
export PORT="${PORT:-8080}"

./station --db-path="/data/station.db" --port="$PORT" &
STATION_PID=$!

echo "✅ Station started (PID: $STATION_PID)"
echo "📊 Health check: http://localhost:$PORT/health"

# Wait for either process to exit
wait "$STATION_PID"
exit_code=$?

# If Station exits, cleanup and exit with same code
cleanup
exit $exit_code