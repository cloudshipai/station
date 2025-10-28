# CICD Build Checklist

## When This Broke

The CICD Docker workflow broke when the UI was added to Station (around commit `1c2cc567` on Oct 19, 2025) but the workflow at `.github/workflows/docker.yml` was never updated to include the UI build steps. The local `make rebuild-all` command was working correctly, but CICD continued using a simpler build that only compiled the Go binary without the embedded UI.

**Symptoms**: Docker images built by CICD had no web UI on port 8585, making them appear broken.

## Root Cause

Station embeds a React/Vite UI using Go's build tags (`-tags ui`). The binary can be built two ways:

1. **Without UI** (`go build`): Just the CLI and API, no web interface
2. **With embedded UI** (`go build -tags ui`): Full binary with web UI served on port 8585

CICD was building without the UI while `make rebuild-all` was building with it.

## How to Prevent This in the Future

### 1. Always Match Local Build in CICD

When modifying the Makefile's `rebuild-all` target, **immediately update** `.github/workflows/docker.yml` to match.

The CICD workflow **MUST** replicate these exact steps from `make rebuild-all`:

```makefile
rebuild-all: local-install-ui
	# This does: build-ui -> copy assets -> go build -tags ui
```

### 2. Required Build Steps for Station

For any CICD workflow that builds Station Docker images, include **ALL** these steps in order:

```yaml
- name: Set up Node.js
  uses: actions/setup-node@v4
  with:
    node-version: '20'
    cache: 'npm'
    cache-dependency-path: ui/package-lock.json

- name: Install UI dependencies
  run: |
    cd ui
    npm install

- name: Build UI
  run: |
    cd ui
    npm run build

- name: Prepare UI assets for embedding
  run: |
    mkdir -p internal/ui/static
    cp -r ui/dist/* internal/ui/static/

- name: Build Station binary with embedded UI
  run: |
    VERSION="${GITHUB_REF_NAME#v}"
    BUILD_TIME="$(date -u +'%Y-%m-%d %H:%M:%S UTC')"
    LDFLAGS="-X 'station/internal/version.Version=${VERSION}' -X 'station/internal/version.BuildTime=${BUILD_TIME}'"

    GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -tags ui -o dist/station_linux_amd64_v1/stn ./cmd/main
    GOOS=linux GOARCH=arm64 go build -ldflags "${LDFLAGS}" -tags ui -o dist/station_linux_arm64/stn ./cmd/main
```

### 3. Critical Build Flags

Never forget these when building Station:

- **`-tags ui`**: Enables embedded UI (without this, binary has no web interface)
- **`-ldflags`**: Injects version and build time into binary
- **UI assets in `internal/ui/static/`**: Must be present before Go build runs

### 4. Testing CICD Changes

Before merging Makefile or CICD changes:

1. Build locally: `make rebuild-all`
2. Build with Docker: `docker build -t test-station .`
3. Run and test UI: `docker run -p 8585:8585 test-station` → `curl localhost:8585` should return HTML
4. If adding new build steps to Makefile, update CICD workflow immediately

### 5. Where Builds Happen

Station has multiple build paths that all need to stay in sync:

| Build Path | Location | Must Include UI Steps |
|------------|----------|----------------------|
| Local development | `make rebuild-all` | ✅ Yes |
| Docker CICD | `.github/workflows/docker.yml` | ✅ Yes |
| GoReleaser | `.goreleaser.yml` | ✅ Yes (if used) |
| Manual Docker | `Dockerfile` | ✅ Yes |

## Quick Test to Verify Build

After any build (local or CICD):

```bash
# For local binary
./dist/station_linux_amd64_v1/stn up --provider openai
curl http://localhost:8585  # Should return HTML

# For Docker image
docker run -p 8585:8585 <image>
curl http://localhost:8585  # Should return HTML
```

If `curl` returns nothing or connection refused, the UI wasn't embedded.

## Related Files

- **Makefile**: `rebuild-all`, `build-with-ui`, `build-ui` targets
- **CICD**: `.github/workflows/docker.yml`
- **UI Source**: `ui/` directory (Vite/React app)
- **UI Embed**: `internal/ui/static/` (where Vite output goes)
- **Go Embed**: `internal/ui/handler.go` (likely contains `//go:embed` directive)

## Fixed In

- **Commit**: `4023a51d` (Oct 28, 2025)
- **PR**: (if applicable)
- **Issue**: "Docker images built in CICD missing embedded UI"
