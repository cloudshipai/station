# PRD: Browser-Based Variable Input for `stn sync`

**Status**: In Development  
**Author**: CloudShip Team  
**Created**: 2026-01-04  
**Last Updated**: 2026-01-04

---

## Executive Summary

Add a `--browser` flag to `stn sync` that opens the Station UI in a browser for secure credential/variable input. This enables LLM agents (like OpenCode, Claude Code) to orchestrate MCP server installations without ever seeing sensitive credentials.

---

## Problem Statement

### Current State

When running `stn sync` with MCP templates that require variables (API keys, tokens, etc.), users must input credentials directly in the terminal:

```bash
$ stn sync production

🔧 Missing Variables Detected
📝 OPENAI_API_KEY: sk-xxxxx  # Visible in terminal history
📝 DATABASE_URL: postgres://user:pass@...
```

### Problems

1. **LLM Security Risk**: When AI coding assistants run `stn sync`, they see credentials in the terminal output
2. **Poor UX**: Basic `fmt.Scanln()` prompts with no validation, no password masking for non-secret fields
3. **No Reuse**: Existing beautiful SyncModal UI in Station is only accessible via the web interface

### User Stories

1. **As an AI coding assistant user**, I want to run `stn sync --browser` so the AI can orchestrate MCP installations while I securely enter credentials in a separate browser window
2. **As a developer**, I want a better UX for entering multiple variables with proper form validation
3. **As a security-conscious user**, I want my credentials to never appear in terminal output or LLM context windows

---

## Solution

### Overview

Add `stn sync --browser` flag that:
1. Starts an interactive sync via the existing API
2. Opens the Station UI in the default browser
3. CLI polls for completion while user enters variables in browser
4. Variables are saved to `variables.yml` (existing behavior)
5. CLI reports success/failure when done

### User Flow

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Terminal                                                                    │
├────────────────────────────────────────────────────────────────────────────┤
│ $ stn sync production --browser                                            │
│                                                                            │
│ 🌐 Opening browser for variable configuration...                           │
│ ⏳ Waiting for variable input in browser (5 min timeout)...                │
│                                                                            │
│ If browser didn't open, visit:                                             │
│ http://localhost:8585/sync/production?sync_id=sync_1704389234567890        │
│                                                                            │
│ ... (user enters variables in browser) ...                                 │
│                                                                            │
│ ✅ Variables configured via browser                                        │
│ ✅ Sync completed: 3 agents, 2 MCP servers connected                       │
└────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────────┐
│ Browser: http://localhost:8585/sync/production?sync_id=...                 │
├────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │  🔧 Configure Variables for: production                              │  │
│  │                                                                      │  │
│  │  OPENAI_API_KEY *                                                    │  │
│  │  ┌────────────────────────────────────────────────────────────────┐  │  │
│  │  │ ••••••••••••••••••••••••••••••                                 │  │  │
│  │  └────────────────────────────────────────────────────────────────┘  │  │
│  │                                                                      │  │
│  │  DATABASE_URL                                                        │  │
│  │  ┌────────────────────────────────────────────────────────────────┐  │  │
│  │  │ postgres://localhost:5432/mydb                                 │  │  │
│  │  └────────────────────────────────────────────────────────────────┘  │  │
│  │                                                                      │  │
│  │  ┌────────────────────────────────────────────────────────────────┐  │  │
│  │  │                      Continue Sync                             │  │  │
│  │  └────────────────────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                            │
│  After completion:                                                         │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │  ✅ Sync Complete!                                                   │  │
│  │  Variables have been configured and saved.                           │  │
│  │  You can close this window and return to the terminal.               │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## Technical Design

### Architecture

```
┌─────────────┐     POST /sync/interactive      ┌─────────────────┐
│   CLI       │ ─────────────────────────────▶  │  Station API    │
│             │     { environment: "prod" }     │                 │
│             │ ◀─────────────────────────────  │                 │
│             │     { sync_id: "sync_xxx" }     │                 │
└──────┬──────┘                                 └────────┬────────┘
       │                                                 │
       │  1. Open browser                                │
       │  http://localhost:8585/sync/prod?sync_id=xxx   │
       ▼                                                 │
┌─────────────┐                                          │
│  Browser    │ ◀────────────────────────────────────────┘
│  (Station   │     Renders SyncModal, polls status
│   React UI) │
└──────┬──────┘
       │
       │  2. User enters variables, submits
       │  POST /sync/variables { sync_id, variables }
       │
       ▼
┌─────────────┐     GET /sync/status/{id}       ┌─────────────────┐
│   CLI       │ ─────────────────────────────▶  │  Station API    │
│  (polling)  │     every 1 second              │                 │
│             │ ◀─────────────────────────────  │                 │
│             │     { status: "completed" }     │                 │
└─────────────┘                                 └─────────────────┘
```

### Components

| Component | File | Change Type |
|-----------|------|-------------|
| CLI flag | `cmd/main/main.go` | Modify |
| Sync handler | `cmd/main/commands.go` | Modify |
| Browser sync service | `internal/services/browser_sync.go` | **New** |
| Sync page | `ui/src/components/sync/SyncPage.tsx` | **New** |
| App routes | `ui/src/App.tsx` | Modify |
| Sync modal | `ui/src/components/sync/SyncModal.tsx` | Modify |

### API Endpoints (Existing - No Changes)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/sync/interactive` | POST | Start interactive sync, returns sync_id |
| `/api/v1/sync/status/:id` | GET | Poll sync status |
| `/api/v1/sync/variables` | POST | Submit variable values |

### New CLI Flag

```bash
stn sync <environment> [flags]

Flags:
  --browser    Open browser for secure variable input (useful for LLM agents)
  --dry-run    Show what would change without making changes
  -i, --interactive    Prompt for missing variables (default: true)
```

---

## Requirements

### Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1 | `stn sync --browser` opens default browser to Station UI | P0 |
| FR-2 | CLI polls API and waits for sync completion | P0 |
| FR-3 | Variables entered in browser are saved to `variables.yml` | P0 |
| FR-4 | 5-minute timeout with clear error message | P0 |
| FR-5 | Fallback URL displayed if browser fails to open | P0 |
| FR-6 | Browser shows completion message when done | P1 |
| FR-7 | CLI shows sync results (agents/MCPs processed) | P1 |

### Non-Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| NFR-1 | No credentials visible in CLI output | P0 |
| NFR-2 | Reuse existing SyncModal component | P0 |
| NFR-3 | Reuse existing sync API endpoints | P0 |
| NFR-4 | Cross-platform browser opening (macOS, Linux, Windows) | P0 |

### Out of Scope (v1)

- `STN_SYNC_BROWSER=true` environment variable (Phase 2)
- Auto-detect TTY and suggest `--browser` (Phase 2)
- Remote browser support (only localhost for v1)

---

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Station server not running | Error: "Station server not running. Start with: stn serve" |
| Browser fails to open | Print URL and continue waiting |
| User closes browser without completing | CLI times out after 5 minutes |
| No variables needed | Sync completes immediately, browser shows success |
| Network error during polling | Retry with exponential backoff |
| Sync fails | CLI shows error message from API |

---

## Security Considerations

1. **Credentials never in CLI output**: All variable values entered only in browser
2. **Localhost only**: Browser opens to `localhost`, no remote access
3. **Sync ID is ephemeral**: Generated per-session, cleaned up after 3 seconds
4. **HTTPS not required**: Localhost communication is acceptable for local dev

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Feature adoption | 20% of interactive syncs use `--browser` within 30 days |
| LLM agent compatibility | Works with OpenCode, Claude Code, Cursor |
| User satisfaction | No credential exposure incidents reported |

---

## Implementation Plan

### Phase 1: Core Implementation (This PR)

1. Add `--browser` flag to CLI
2. Create `browser_sync.go` service
3. Create `SyncPage.tsx` component
4. Add route to `App.tsx`
5. Enhance `SyncModal.tsx` for standalone mode

### Phase 2: Enhancements (Future)

1. `STN_SYNC_BROWSER=true` env var for default browser mode
2. Auto-suggest `--browser` when TTY not detected
3. Better error messages for common failures

---

## Appendix

### Related Files

- `station/internal/api/v1/sync_interactive.go` - Existing interactive sync API
- `station/ui/src/components/sync/SyncModal.tsx` - Existing sync modal UI
- `station/internal/provider/anthropic_oauth.go` - Reference for browser opening pattern
- `station/cmd/main/commands.go` - Existing sync command

### References

- [OAuth-style browser flow in Station](../internal/provider/anthropic_oauth.go)
- [Existing SyncModal component](../../ui/src/components/sync/SyncModal.tsx)
