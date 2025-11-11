# Jaeger Quick Reference Card

## 🚀 Quick Commands

### Start with Jaeger
```bash
# Server mode (manual)
stn serve --jaeger

# Stdio mode (auto-enabled)
stn stdio

# Docker mode (auto-enabled)
stn up
```

### Access Jaeger
```bash
# Open Jaeger UI
open http://localhost:16686

# Check if running
curl -I http://localhost:16686
```

### Disable Jaeger
```bash
# Stdio mode
stn stdio --jaeger=false

# Docker mode
stn up --jaeger=false

# Server mode (already disabled by default)
stn serve
```

## 🔧 Environment Variables

```bash
# Enable for all modes
export STATION_AUTO_JAEGER=true

# Then run any command
stn serve    # Jaeger enabled
stn stdio    # Jaeger enabled
stn up       # Jaeger enabled
```

## 📊 Default Behaviors

| Command | Jaeger | Override |
|---------|--------|----------|
| `stn serve` | OFF | `--jaeger` |
| `stn stdio` | **ON** | `--jaeger=false` |
| `stn up` | **ON** | `--jaeger=false` |

## 🔍 Ports

| Port | Service |
|------|---------|
| 16686 | Jaeger UI |
| 4318 | OTLP HTTP |
| 4317 | OTLP gRPC |

## 📁 Storage

```bash
# Traces stored at:
~/.local/share/station/jaeger-data/

# View storage
ls -la ~/.local/share/station/jaeger-data/
```

## 🧪 Quick Test

```bash
# Build
go build -o /tmp/station ./cmd/main

# Test stdio (30 seconds)
/tmp/station stdio &
sleep 3
curl http://localhost:16686
fg  # Ctrl+C

# Test serve (30 seconds)
/tmp/station serve --jaeger &
sleep 3
curl http://localhost:16686
fg  # Ctrl+C
```

## 🐛 Troubleshooting

### Port already in use
```bash
# Check what's using port
lsof -i :16686

# Stop existing Jaeger
docker ps | grep jaeger
docker stop <container-id>
```

### Jaeger not starting
```bash
# Check Docker is running
docker ps

# Check Dagger
dagger version

# View logs
docker logs <jaeger-container-id>
```

### No traces appearing
```bash
# Check OTLP endpoint
echo $OTEL_EXPORTER_OTLP_ENDPOINT
# Should be: http://localhost:4318

# Note: Phase 2 (OTEL SDK) required for actual traces
```

## 📖 Documentation

- **Complete Guide**: `docs/features/JAEGER_AUTO_LAUNCH.md`
- **Quick Start**: `JAEGER_QUICK_START.md`
- **Implementation**: `JAEGER_IMPLEMENTATION_SUMMARY.md`
- **Stdio Details**: `JAEGER_STDIO_INTEGRATION.md`

## 🎯 Next Steps

1. **Test**: Run quick tests above
2. **Phase 2**: Integrate OTEL SDK (4-6h)
3. **Phase 3**: Reports system (40-56h)

---
*Last Updated: 2025-11-11*
