#!/bin/bash

# Simplified test to verify Litestream replication works with SQLite
set -e

echo "🧪 Testing Litestream Replication & Restoration"
echo "=============================================="

# Clean up any previous test
rm -rf /tmp/litestream-test /tmp/test-backup

# Create test directories
mkdir -p /tmp/litestream-test /tmp/test-backup
cd /tmp/litestream-test

# Download Litestream if needed
if ! command -v litestream &> /dev/null; then
    echo "📥 Installing Litestream..."
    curl -s https://api.github.com/repos/benbjohnson/litestream/releases/latest | \
        grep "browser_download_url.*linux-amd64.tar.gz" | \
        cut -d '"' -f 4 | \
        xargs curl -L | tar -xz
    export PATH="$PWD:$PATH"
fi

echo "✅ Litestream available"

# Create test Litestream config
cat > litestream.yml << 'EOF'
dbs:
  - path: ./test.db
    replicas:
      - type: file
        path: /tmp/test-backup
        sync-interval: 1s
        retention: 1h
EOF

# Create initial test database with data
echo "🗄️  Creating test database..."
sqlite3 test.db << 'EOF'
CREATE TABLE test_agents (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_agents (name) VALUES ('agent-1');
INSERT INTO test_agents (name) VALUES ('agent-2');
INSERT INTO test_agents (name) VALUES ('agent-3');
EOF

echo "📊 Initial database content:"
sqlite3 test.db "SELECT id, name, created_at FROM test_agents;"

# Start Litestream replication
echo ""
echo "🔄 Starting Litestream replication..."
litestream replicate -config litestream.yml &
LITESTREAM_PID=$!

# Wait for replication to start
sleep 3

# Check backup directory
echo "📁 Backup directory contents:"
ls -la /tmp/test-backup/ || echo "No backup files yet"

# Add more data to trigger replication
echo ""
echo "📝 Adding data during replication..."
sqlite3 test.db << 'EOF'
INSERT INTO test_agents (name) VALUES ('realtime-agent');
UPDATE test_agents SET name = 'updated-agent-1' WHERE id = 1;
EOF

echo "📊 Updated database content:"
sqlite3 test.db "SELECT id, name FROM test_agents;"

# Wait for replication
sleep 3

# Check final backup state
echo "📁 Final backup directory:"
ls -la /tmp/test-backup/

# Stop Litestream
kill $LITESTREAM_PID
sleep 1

# Test restoration
echo ""
echo "🔄 Testing database restoration..."

# Get original count
ORIGINAL_COUNT=$(sqlite3 test.db "SELECT COUNT(*) FROM test_agents;")
echo "📊 Original agent count: $ORIGINAL_COUNT"

# Remove original database
rm test.db
echo "🗑️  Removed original database"

# Restore from backup
echo "📦 Restoring from backup..."
if litestream restore -config litestream.yml ./test.db; then
    echo "✅ Restoration successful"
    
    # Verify restored data
    RESTORED_COUNT=$(sqlite3 test.db "SELECT COUNT(*) FROM test_agents;")
    echo "📊 Restored agent count: $RESTORED_COUNT"
    
    if [ "$ORIGINAL_COUNT" = "$RESTORED_COUNT" ]; then
        echo "✅ Data integrity verified!"
        echo "📋 Restored content:"
        sqlite3 test.db "SELECT id, name FROM test_agents;"
        
        # Check specific data
        if sqlite3 test.db "SELECT name FROM test_agents WHERE name = 'realtime-agent';" | grep -q "realtime-agent"; then
            echo "✅ Real-time changes preserved!"
        else
            echo "❌ Real-time changes lost"
            exit 1
        fi
        
        if sqlite3 test.db "SELECT name FROM test_agents WHERE id = 1;" | grep -q "updated-agent-1"; then
            echo "✅ Updates preserved!"
        else
            echo "❌ Updates lost"
            exit 1
        fi
        
    else
        echo "❌ Data count mismatch: expected $ORIGINAL_COUNT, got $RESTORED_COUNT"
        exit 1
    fi
else
    echo "❌ Restoration failed"
    exit 1
fi

# Cleanup
cd /
rm -rf /tmp/litestream-test /tmp/test-backup

echo ""
echo "🎉 LITESTREAM TEST RESULTS:"
echo "=========================="
echo "✅ Database replication: PASSED"
echo "✅ Real-time sync: PASSED"  
echo "✅ Database restoration: PASSED"
echo "✅ Data integrity: PASSED"
echo ""
echo "💡 Litestream is working correctly!"
echo "🚀 Station's GitOps SQLite persistence will work in production"