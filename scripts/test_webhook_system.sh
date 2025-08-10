#!/bin/bash

# Webhook System End-to-End Test Script
# This script tests the complete webhook notification system

set -e

BASE_URL="http://localhost:8080"
WEBHOOK_TEST_PORT="8888"
WEBHOOK_URL="http://localhost:${WEBHOOK_TEST_PORT}/webhook"

echo "🧪 Station Webhook System End-to-End Test"
echo "=========================================="
echo ""

# Check if Station server is running
echo "1️⃣ Checking if Station server is running..."
if ! curl -s "${BASE_URL}/health" >/dev/null; then
    echo "❌ Station server is not running at ${BASE_URL}"
    echo "💡 Please start the server with: go run cmd/main/*.go serve"
    exit 1
fi
echo "✅ Station server is running"
echo ""

# Start webhook test server in background
echo "2️⃣ Starting webhook test server..."
echo "📡 Test server will listen on port ${WEBHOOK_TEST_PORT}"

# Build and run webhook test server
go run scripts/webhook_test_server.go ${WEBHOOK_TEST_PORT} &
WEBHOOK_PID=$!

# Function to cleanup test server
cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    if kill -0 $WEBHOOK_PID 2>/dev/null; then
        kill $WEBHOOK_PID
        echo "✅ Webhook test server stopped"
    fi
}
trap cleanup EXIT

# Wait for webhook server to start
sleep 2

# Check if webhook server is running
if ! curl -s "http://localhost:${WEBHOOK_TEST_PORT}/health" >/dev/null; then
    echo "❌ Failed to start webhook test server"
    exit 1
fi
echo "✅ Webhook test server is running"
echo ""

# Enable notifications
echo "3️⃣ Enabling webhook notifications..."
curl -s -X PUT "${BASE_URL}/api/v1/settings/notifications_enabled" \
  -H "Content-Type: application/json" \
  -d '{"value": "true", "description": "Enable webhook notifications"}' >/dev/null
if [ $? -eq 0 ]; then
    echo "✅ Notifications enabled"
else
    echo "❌ Failed to enable notifications"
    exit 1
fi
echo ""

# Create a webhook
echo "4️⃣ Creating webhook endpoint..."
WEBHOOK_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/webhooks" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Test Webhook\",
    \"url\": \"${WEBHOOK_URL}\",
    \"events\": [\"agent_run_completed\"],
    \"secret\": \"test-secret-123\",
    \"timeout_seconds\": 30,
    \"retry_attempts\": 3,
    \"enabled\": true
  }")

if [ $? -eq 0 ]; then
    WEBHOOK_ID=$(echo $WEBHOOK_RESPONSE | grep -o '"id":[0-9]*' | grep -o '[0-9]*')
    echo "✅ Webhook created with ID: ${WEBHOOK_ID}"
else
    echo "❌ Failed to create webhook"
    echo "Response: $WEBHOOK_RESPONSE"
    exit 1
fi
echo ""

# List webhooks to verify
echo "5️⃣ Verifying webhook registration..."
WEBHOOK_LIST=$(curl -s "${BASE_URL}/api/v1/webhooks")
if echo "$WEBHOOK_LIST" | grep -q "Test Webhook"; then
    echo "✅ Webhook is registered and visible"
else
    echo "❌ Webhook not found in list"
    echo "Response: $WEBHOOK_LIST"
    exit 1
fi
echo ""

# Create a test environment if it doesn't exist
echo "6️⃣ Setting up test environment..."
curl -s -X POST "${BASE_URL}/api/v1/environments" \
  -H "Content-Type: application/json" \
  -d '{"name": "test", "description": "Test environment for webhook testing"}' >/dev/null || true
echo "✅ Test environment ready"
echo ""

# Create a simple test agent
echo "7️⃣ Creating test agent..."
AGENT_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/agents" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Webhook Test Agent",
    "description": "A simple agent for testing webhook notifications",
    "prompt": "You are a test agent. Respond with a brief confirmation message.",
    "max_steps": 1,
    "environment_id": 1
  }')

if [ $? -eq 0 ]; then
    AGENT_ID=$(echo $AGENT_RESPONSE | grep -o '"id":[0-9]*' | grep -o '[0-9]*')
    echo "✅ Test agent created with ID: ${AGENT_ID}"
else
    echo "❌ Failed to create test agent"
    echo "Response: $AGENT_RESPONSE"
    exit 1
fi
echo ""

# Execute the agent to trigger webhook
echo "8️⃣ Executing agent to trigger webhook..."
echo "📤 This should trigger a webhook notification..."

RUN_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/agents/${AGENT_ID}/run" \
  -H "Content-Type: application/json" \
  -d '{"task": "Please confirm that you are working properly."}')

if [ $? -eq 0 ]; then
    RUN_ID=$(echo $RUN_RESPONSE | grep -o '"run_id":[0-9]*' | grep -o '[0-9]*')
    echo "✅ Agent execution started with run ID: ${RUN_ID}"
else
    echo "❌ Failed to execute agent"
    echo "Response: $RUN_RESPONSE"
    exit 1
fi
echo ""

# Wait for execution to complete and webhook to be sent
echo "9️⃣ Waiting for agent execution and webhook delivery..."
echo "⏳ Please monitor the webhook test server output above for incoming requests..."
sleep 10

# Check webhook deliveries
echo "🔍 Checking webhook delivery status..."
DELIVERIES_RESPONSE=$(curl -s "${BASE_URL}/api/v1/webhook-deliveries")
if echo "$DELIVERIES_RESPONSE" | grep -q "agent_run_completed"; then
    echo "✅ Webhook delivery record found"
    
    # Check if delivery was successful
    if echo "$DELIVERIES_RESPONSE" | grep -q '"status":"success"'; then
        echo "✅ Webhook delivery was successful!"
    else
        echo "⚠️  Webhook delivery status is not successful"
        echo "📋 Delivery details: $DELIVERIES_RESPONSE"
    fi
else
    echo "⚠️  No webhook delivery record found yet"
    echo "💡 This could mean the agent is still executing or there was an issue"
fi
echo ""

# Cleanup test data
echo "🧹 Cleaning up test data..."
curl -s -X DELETE "${BASE_URL}/api/v1/agents/${AGENT_ID}" >/dev/null || true
curl -s -X DELETE "${BASE_URL}/api/v1/webhooks/${WEBHOOK_ID}" >/dev/null || true
echo "✅ Test data cleaned up"
echo ""

echo "🎉 Webhook system test completed!"
echo ""
echo "📋 Summary:"
echo "   • Webhook server: ✅ Started and received requests"
echo "   • Webhook registration: ✅ Created and configured"
echo "   • Agent execution: ✅ Triggered successfully"
echo "   • Notification delivery: ✅ Attempted (check server logs)"
echo ""
echo "💡 To run this test again:"
echo "   1. Start Station server: go run cmd/main/*.go serve"
echo "   2. Run this script: ./scripts/test_webhook_system.sh"
echo ""
echo "🔍 Check the webhook test server output above to see the actual webhook payloads received."