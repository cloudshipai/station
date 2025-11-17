# Build Test Fixtures

This directory contains test fixtures for build and deployment testing.

## Structure

- `sample-environment/` - A complete test environment with:
  - `agents/` - Two test agents for multi-agent verification
  - `template.json` - MCP server configuration (filesystem only)
  - `variables.yml` - Template variables (excluded from bundles)

## Usage

### Unit Tests
```go
envPath := filepath.Join("test", "fixtures", "build", "sample-environment")
bundleService := services.NewBundleService()
bundleBytes, err := bundleService.CreateBundle(envPath)
```

### Integration Tests
```bash
# Create bundle from fixtures
stn bundle create test/fixtures/build/sample-environment

# Build Docker image
stn build test --env-path test/fixtures/build/sample-environment
```

### E2E Tests
```bash
# Full workflow test
./test/scripts/e2e-build-test.sh
```

## Verification

After deployment, verify:
1. ✅ 2 agents are synced into database
2. ✅ Filesystem MCP server is connected
3. ✅ Agents can execute file operations
4. ✅ variables.yml is NOT in bundle (security)
