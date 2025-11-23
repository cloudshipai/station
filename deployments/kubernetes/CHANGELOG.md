# Kubernetes Deployment Changelog

## 2025-11-23 - GEMINI_API_KEY Bug Fix

### Fixed
- **BenchmarkService Lazy Initialization**: Station no longer requires `GEMINI_API_KEY` when using OpenAI as the AI provider
  - Modified `internal/services/benchmark_service.go` to use lazy initialization pattern
  - BenchmarkService only initializes when first used (benchmark evaluation requested)
  - Server starts successfully with only `OPENAI_API_KEY` configured
  - Clear error messages if benchmark evaluation attempted without proper credentials

### Updated
- `secret.yaml`: GEMINI_API_KEY is now optional (only needed for Gemini provider)
- `README.md`: Updated documentation to reflect optional GEMINI_API_KEY
- `GEMINI_API_KEY_BUG.md`: Marked as fixed with implementation details

### Breaking Changes
None - This is a backwards-compatible fix. Existing deployments with GEMINI_API_KEY will continue to work.

### Testing
```bash
# Verified Station starts without GEMINI_API_KEY
unset GEMINI_API_KEY GOOGLE_API_KEY
export OPENAI_API_KEY="your-key"
stn serve --local
# ✅ Server starts successfully
```

### Migration Guide
If you were using the workaround (`GEMINI_API_KEY=dummy`):

1. Update to Station v0.1.0+
2. Remove `GEMINI_API_KEY` from your secret (if using OpenAI)
3. Redeploy

```bash
# Update secret
kubectl edit secret station-secrets -n station
# Remove GEMINI_API_KEY line

# Restart deployment to pick up changes
kubectl rollout restart deployment/station -n station
```

---

## 2025-11-23 - Initial Kubernetes Deployment

### Added
- Complete Kubernetes deployment manifests
  - `namespace.yaml` - Station namespace
  - `secret.yaml` - API keys and encryption key
  - `configmap.yaml` - Non-sensitive configuration
  - `pvc.yaml` - Persistent volume for database (REQUIRED)
  - `deployment.yaml` - Station deployment with health checks
  - `service.yaml` - ClusterIP and LoadBalancer services
  - `README.md` - Comprehensive deployment guide
  - `GEMINI_API_KEY_BUG.md` - Bug analysis and fix documentation

### Key Features
- Production-ready manifests with health checks
- PersistentVolume support (required for SQLite database)
- ConfigMap/Secret separation for security
- Liveness and readiness probes
- Resource limits and requests
- k3s specific notes and configurations

### Requirements
- Persistent volume mounted to `/data` (MANDATORY)
- STATION_ENCRYPTION_KEY for secrets storage
- AI provider API keys (OpenAI, Gemini, etc.)
- STN_DEV_MODE=true for API server access

### Known Limitations
- Single instance only (SQLite limitation)
- ReadWriteOnce PVC access mode
- Recreate deployment strategy required
