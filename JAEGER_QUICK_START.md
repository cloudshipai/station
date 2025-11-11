# Jaeger Quick Start Guide

## 🎯 Quick Test (5 minutes)

### Step 1: Build Station
```bash
cd /home/epuerta/projects/hack/station
go build -o /tmp/station ./cmd/main
```

### Step 2: Start Server with Jaeger
```bash
/tmp/station serve --jaeger --database ~/.config/station/station.db
```

**Expected Output**:
```
🔍 Launching Jaeger (background service)...
   ✅ Jaeger UI: http://localhost:16686
   ✅ OTLP endpoint: http://localhost:4318
   ✅ Traces persist to: /home/user/.local/share/station/jaeger-data
...
✅ Station is running!
🔧 MCP Server: http://localhost:8586/mcp
🤖 Dynamic Agent MCP: http://localhost:3031/mcp (environment: default)
🌐 API Server: http://localhost:8585
```

### Step 3: Access Jaeger UI
```bash
open http://localhost:16686
# Or visit in browser: http://localhost:16686
```

### Step 4: Run an Agent (Generate Traces)
```bash
# In another terminal
/tmp/station agent list
/tmp/station agent call <agent-id> "hello world"
```

### Step 5: View Traces in Jaeger
1. Go to http://localhost:16686
2. Select service: `station-server`
3. Click "Find Traces"
4. Explore trace details

### Step 6: Stop Server (Verify Graceful Shutdown)
```bash
# In server terminal, press Ctrl+C
```

**Expected Output**:
```
🛑 Received shutdown signal, gracefully shutting down...
🔧 Shutting down MCP server...
🔧 MCP server stopped gracefully
🤖 Shutting down Dynamic Agent MCP server...
🤖 Dynamic Agent MCP server stopped gracefully
🧹 Stopping Jaeger service...
   ✅ Jaeger stopped
✅ All servers stopped gracefully
```

## 🔌 Stdio Mode Test (5 minutes)

### Step 1: Start Stdio Mode with Jaeger
```bash
/tmp/station stdio --jaeger
```

**Expected Output**:
```
🔍 Jaeger UI: http://localhost:16686
🔍 OTLP endpoint: http://localhost:4318
🚀 Station MCP Server starting in stdio mode
Local mode: true
Agent execution: enabled
Ready for MCP communication via stdin/stdout
🌐 Management channel active - Station remains available for CloudShip control
```

### Step 2: Access Jaeger UI
```bash
# In another terminal
open http://localhost:16686
```

### Step 3: Test MCP Communication
```bash
# In another terminal, send MCP request
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | /tmp/station stdio --jaeger=false
```

### Step 4: Stop Server
```bash
# Press Ctrl+C in stdio terminal
```

**Expected Output**:
```
🛑 Received termination signal, shutting down...
🛑 Shutting down API server...
🛑 Shutting down remote control service...
🛑 Shutting down Jaeger...
   ✅ Jaeger stopped
```

## 🐳 Docker Test (5 minutes)

### Step 1: Build Container
```bash
cd /home/epuerta/projects/hack/station
docker build -t station-server:latest .
```

### Step 2: Start with Docker
```bash
stn up --jaeger
```

### Step 3: Verify Jaeger UI
```bash
open http://localhost:16686
```

### Step 4: Check Logs
```bash
stn logs
# Should show "Launching Jaeger" and "Jaeger UI: http://localhost:16686"
```

### Step 5: Stop Container
```bash
stn down
```

## 🧪 Run Test Script

```bash
cd /home/epuerta/projects/hack/station
./dev-workspace/test-scripts/test-jaeger-service.sh
```

## 🔍 Verify Components

### Check if Jaeger is Running
```bash
# Should return HTTP 200
curl -I http://localhost:16686

# Should show Jaeger container
docker ps | grep jaeger
```

### Check OTLP Endpoint
```bash
# Should be accessible (may return 404, that's OK)
curl http://localhost:4318/v1/traces
```

### Check Persistent Storage
```bash
ls -la ~/.local/share/station/jaeger-data/
# Should show badger/ directory with data files
```

## 🚨 Troubleshooting

### Port Already in Use
```bash
# Check what's using port 16686
lsof -i :16686

# Kill existing Jaeger
docker ps | grep jaeger
docker stop <container-id>
```

### Dagger Connection Failed
```bash
# Check Docker is running
docker ps

# Restart Docker daemon
sudo systemctl restart docker
```

### No Traces Appearing
```bash
# Check OTEL endpoint is set
echo $OTEL_EXPORTER_OTLP_ENDPOINT
# Should output: http://localhost:4318

# Verify agent execution creates spans
# (This requires OTEL SDK integration - Phase 2)
```

## ✅ Success Criteria

- [x] Server starts without errors
- [x] Jaeger UI accessible at http://localhost:16686
- [x] No "port already in use" errors
- [x] Graceful shutdown works (Ctrl+C)
- [x] Persistent storage created (~/.local/share/station/jaeger-data/)
- [ ] Traces appear in Jaeger UI (requires Phase 2 - OTEL SDK)

## 📋 Testing Checklist

- [ ] CLI server with `--jaeger` flag works
- [ ] Environment variable `STATION_AUTO_JAEGER=true` works
- [ ] Docker container `stn up --jaeger` works
- [ ] Jaeger UI accessible and loads
- [ ] Server shutdown is clean (<5 seconds)
- [ ] Persistent storage survives restart
- [ ] Port conflict detection works (start Jaeger twice)

## 🎯 Next Phase: OTEL SDK Integration

Once basic Jaeger works, implement OpenTelemetry SDK:

```bash
# Add OTEL dependencies
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/trace
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp

# Instrument agent execution
# See: docs/OTEL_IMPLEMENTATION_PLAN.md
```

---
**Quick Commands**:
```bash
# Build and run
go build -o /tmp/station ./cmd/main && /tmp/station serve --jaeger

# Test and verify
curl http://localhost:16686 && open http://localhost:16686
```
