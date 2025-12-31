# PRD: `stn up <environment>` - Local Environment Bundling

**Status**: In Progress  
**Author**: Station Team  
**Date**: 2025-12-31  
**Version**: 0.1

---

## Problem Statement

Currently, `stn up` requires either:
1. `--bundle <cloudship-uuid>` - Download and run a bundle from CloudShip
2. `--bundle ./path.tar.gz` - Run a local bundle file

There's no easy way to test a local environment directory in the containerized `stn up` workflow. Developers have to:
1. Manually run `stn bundle create my-env -o my-env.tar.gz`
2. Then run `stn up --bundle ./my-env.tar.gz`
3. Manually pass `-e` flags for each variable in `variables.yml`

This friction slows down the development → test → iterate cycle.

---

## Solution: `stn up <environment>`

Add support for a positional argument that bundles a local environment on-the-fly and serves it with automatic variable injection.

### New Command Syntax

```bash
# Bundle local environment and serve (auto-reads variables.yml)
stn up <environment>

# Examples
stn up default
stn up my-finops-bundle
stn up aws-k8s-grafana
```

### Behavior Comparison

| Command | Variables | Use Case |
|---------|-----------|----------|
| `stn up --bundle <uuid>` | Explicit `-e` flags only | Production/CloudShip bundles |
| `stn up --bundle ./file.tar.gz` | Explicit `-e` flags only | Testing packaged bundles |
| `stn up <environment>` | Auto-read from `variables.yml` | Local development |

---

## Detailed Design

### Flow: First Run with Environment

```
stn up my-environment
    │
    ├── 1. Validate environment exists
    │      ~/.config/station/environments/my-environment/
    │
    ├── 2. Read variables.yml
    │      Parse all key-value pairs
    │      For each: check host env, use default if not set
    │
    ├── 3. Bundle on-the-fly
    │      Create temp tarball (like `stn bundle create`)
    │
    ├── 4. Import host config.yaml
    │      Copy ~/.config/station/config.yaml into container
    │      Rewrite localhost → host.docker.internal
    │
    ├── 5. Install bundle into volume
    │      Extract to station-config volume
    │
    ├── 6. Store metadata
    │      Write source info to volume for subsequent runs
    │
    └── 7. Start container
         Pass all variables as -e flags
```

### Flow: Subsequent Runs

```
stn up
    │
    ├── Volume exists with valid config?
    │   │
    │   ├── YES → Just start container (fast path)
    │   │         Re-read variables.yml from original source
    │   │         Pass variables as -e flags
    │   │
    │   └── NO → Error: "No configuration found"
    │            "Run 'stn up <environment>' or 'stn up --bundle <id>'"
```

### Flow: Reset and Reconfigure

```
stn down                    → Stop container, keep config in volume
stn down --remove-volume    → Wipe everything (full reset)
stn up <new-environment>    → Fresh configuration
```

### Conflict Handling

| Scenario | Behavior |
|----------|----------|
| `stn up <env>` when volume has same env | Idempotent - just start |
| `stn up <env>` when volume has different env | Error: "Already configured with X. Run 'stn down --remove-volume' first" |
| `stn up --bundle X` when volume configured | Error: "Already configured. Run 'stn down --remove-volume' first" |
| `stn up` with no args, no volume | Error: "Run 'stn up <environment>' or 'stn up --bundle <id>'" |

---

## variables.yml Handling

### Format

```yaml
# Simple key: value format
AWS_ACCOUNT_ID: "123456789012"
AWS_REGION: "us-east-1"
GRAFANA_URL: "http://grafana:3000"
GITHUB_TOKEN: ""  # Empty = must be provided by host env
```

### Resolution Order

1. Host environment variable (if set)
2. Default value from variables.yml
3. Empty string (if not in variables.yml and not in host env)

### Example

```yaml
# variables.yml
AWS_REGION: "us-east-1"
GITHUB_TOKEN: ""
```

```bash
# Host environment
export GITHUB_TOKEN=ghp_xxx

# stn up my-environment passes:
# -e AWS_REGION=us-east-1      (from variables.yml default)
# -e GITHUB_TOKEN=ghp_xxx      (from host env, overrides empty default)
```

---

## Volume Metadata

Store configuration source in volume for subsequent runs:

**File**: `/home/station/.config/station/.stn-up-metadata.json`

```json
{
  "source_type": "environment",
  "source_name": "my-environment",
  "source_path": "/home/user/.config/station/environments/my-environment",
  "variables_file": "variables.yml",
  "configured_at": "2025-12-31T10:00:00Z",
  "stn_version": "0.1.0"
}
```

For bundles:

```json
{
  "source_type": "bundle",
  "source_name": "e26b414a-f076-4135-927f-810bc1dc892a",
  "source_path": "",
  "variables_file": null,
  "configured_at": "2025-12-31T10:00:00Z",
  "stn_version": "0.1.0"
}
```

---

## Implementation Plan

### Phase 1: Core Logic ✅ Designing

**File**: `cmd/main/up.go`

- [ ] Add positional argument parsing for `<environment>`
- [ ] Create `bundleEnvironmentOnTheFly()` function
- [ ] Create `readVariablesYml()` function
- [ ] Create `writeUpMetadata()` / `readUpMetadata()` functions
- [ ] Modify `runUp()` to handle new flow

### Phase 2: Variable Injection

- [ ] Parse variables.yml into map
- [ ] Merge with host environment variables
- [ ] Pass all as `-e` flags to docker run

### Phase 3: Idempotent Restart

- [ ] Check for existing metadata on `stn up` with no args
- [ ] Re-read variables from original source
- [ ] Fast-path container start

### Phase 4: Conflict Detection

- [ ] Detect volume configured with different source
- [ ] Clear error messages with resolution steps

### Phase 5: Documentation

- [ ] Update `docs/station/container-lifecycle.md`
- [ ] Update CLI help text
- [ ] Add examples to `stn up --help`

---

## CLI Changes

### Updated `stn up --help`

```
Start Station server in a Docker container.

Usage:
  stn up [environment] [flags]

Arguments:
  environment    Local environment to bundle and serve (optional)
                 Located at ~/.config/station/environments/<name>/

Examples:
  # Start previously configured container
  stn up

  # Bundle and serve local environment (auto-reads variables.yml)
  stn up my-environment
  stn up default

  # Start with CloudShip bundle (explicit -e flags)
  stn up --bundle e26b414a-f076-4135-927f-810bc1dc892a -e AWS_KEY=xxx

  # Start with local bundle file
  stn up --bundle ./my-bundle.tar.gz -e GITHUB_TOKEN=$GITHUB_TOKEN

  # Reset and reconfigure
  stn down --remove-volume
  stn up new-environment

Flags:
  -w, --workspace string   Workspace directory to mount (default: current directory)
      --bundle string      CloudShip bundle ID, URL, or local file path
  -e, --env strings        Environment variables to pass (required for --bundle)
      --provider string    AI provider (openai, gemini, anthropic, custom)
      --upgrade            Rebuild container image before starting
  -h, --help               help for up
```

---

## Testing Strategy

### Unit Tests

1. `TestReadVariablesYml` - Parse variables.yml correctly
2. `TestMergeVariables` - Host env overrides defaults
3. `TestWriteReadMetadata` - Metadata persistence
4. `TestConflictDetection` - Different environment error

### Integration Tests

1. `TestUpEnvironmentFirstRun` - Full flow with local environment
2. `TestUpIdempotent` - Second `stn up` just starts container
3. `TestUpAfterDown` - Stop and restart preserves config
4. `TestUpAfterRemoveVolume` - Full reset works

---

## Success Criteria

- [ ] `stn up my-env` bundles and starts in one command
- [ ] Variables from variables.yml automatically passed to container
- [ ] `stn up` (no args) restarts previously configured container
- [ ] `stn down` + `stn up` preserves configuration
- [ ] `stn down --remove-volume` + `stn up my-env` fully resets
- [ ] Clear error messages for conflicts

---

## Future Enhancements

1. **Hot reload**: `stn up --watch` to detect environment changes and restart
2. **Variable prompting**: Interactive mode to fill missing required variables
3. **Multi-environment**: Support multiple environments in single container
