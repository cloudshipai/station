# OpenCode WebUI Integration - Product Requirements Document

## Current Status: 🚧 PTY + WebSocket Bridge Implemented - Ready for Testing

### What's Been Completed ✅

#### 1. Architecture Pivot (SSH → WebUI)
- **From**: SSH-based approach with Wish server + Dagger containers
- **To**: Direct WebUI integration with pulsating red "Code" button
- **Reason**: User feedback - "avoid ssh altogether and use the webui"

#### 2. Backend Implementation Complete
- **File**: `internal/api/v1/code.go` (337 lines)
- **PTY Integration**: Uses `github.com/creack/pty` for direct OpenCode process management
- **WebSocket Bridge**: Bidirectional communication between browser and OpenCode TUI
- **Session Management**: Port-based isolation (30000-30099 range) with unique session IDs
- **Process Lifecycle**: Proper cleanup with graceful termination (SIGINT → SIGKILL)

#### 3. Frontend Integration Complete  
- **File**: `ui/src/App.tsx` (updated)
- **xterm.js Integration**: Modern `@xterm/xterm` + `@xterm/addon-fit` packages
- **Tokyo Night Theme**: Custom terminal styling matching Station's theme
- **Session Polling**: Robust 30-second timeout with 1-second intervals
- **WebSocket Connection**: Direct connection to `/api/v1/code/session/{id}/ws`

#### 4. Build System Updated
- **Dependencies**: Added `@xterm/xterm` and `@xterm/addon-fit` to UI package.json
- **CSS Integration**: Added xterm styles to `ui/src/index.css`
- **Make Target**: `make local-install-ui-opencode` builds complete system
- **Binary Embedding**: OpenCode binary embedded with `//go:build opencode` tags

#### 5. Model Flexibility Added
- **Issue**: Hardcoded model validation blocked GPT-5 and custom models
- **Fix**: Removed model name restrictions in `pkg/dotprompt/genkit_executor.go`
- **Result**: Users can now use any model name (GPT-5, custom models, etc.)

### Architecture Overview

```
┌─────────────────┐    HTTP/WS     ┌──────────────────┐    PTY    ┌─────────────┐
│   React WebUI   │ ◄─────────────► │  Go API Server   │ ◄────────► │  OpenCode   │
│   (xterm.js)    │                │  (Gin Router)    │           │   TUI       │
└─────────────────┘                └──────────────────┘           └─────────────┘
      │                                     │                            │
      │ POST /api/v1/code/start            │ startOpenCodeWithPTY()     │ exec.Command()
      │ GET  /api/v1/code/session/:id      │ session management         │ pty.Start()
      │ WS   /api/v1/code/session/:id/ws   │ handleCodeWebSocket()      │ binary extraction
      │                                     │                            │
      └─ Dynamic xterm.js import           └─ Session store in memory   └─ Workspace scoped
```

### Key Files Modified

1. **`internal/api/v1/code.go`** - Complete PTY + WebSocket implementation
2. **`ui/src/App.tsx`** - xterm.js integration with WebSocket connection  
3. **`ui/package.json`** - Added `@xterm/xterm` and `@xterm/addon-fit`
4. **`ui/src/index.css`** - Added xterm CSS import
5. **`pkg/dotprompt/genkit_executor.go`** - Removed model validation restrictions
6. **`DAGGER_STATUS.md`** - Documentation of previous Dagger approach

### What's Ready for Testing

#### Backend API Endpoints ✅
- `POST /api/v1/code/start` - Creates session, extracts binary, starts PTY
- `GET /api/v1/code/session/{id}` - Returns session status and WebSocket URL
- `WS /api/v1/code/session/{id}/ws` - Bidirectional terminal communication
- `POST /api/v1/code/stop` - Stops all active sessions

#### Frontend Integration ✅  
- Red pulsating "Code" button in navigation
- Full-screen terminal interface with Tokyo Night theme
- Proper session polling and WebSocket connection
- Ctrl+C exit functionality
- Terminal resize handling with FitAddon

#### Process Management ✅
- OpenCode binary extraction to `/tmp/opencode-{sessionId}`
- Environment variables (ANTHROPIC_API_KEY, OPENAI_API_KEY, TERM, OPENCODE)
- Workspace creation and scoping
- PTY process monitoring and cleanup

### Next Steps for Future Development

#### 1. Immediate Testing Tasks
- [ ] Test end-to-end flow: button click → terminal appears → OpenCode starts
- [ ] Verify WebSocket bidirectional communication works
- [ ] Test terminal input/output with actual OpenCode TUI
- [ ] Validate Ctrl+C termination and cleanup
- [ ] Test multiple concurrent sessions

#### 2. Known Issues to Address
- **CSS Import Order**: Vite warning about `@import` placement in index.css
- **Bundle Size**: 2MB+ frontend bundle due to xterm.js (consider code splitting)
- **API Keys**: Currently using dummy keys - need proper key management
- **Error Handling**: Need better error messaging for failed sessions

#### 3. Enhancement Opportunities
- **Session Persistence**: Move from in-memory to Redis/database storage
- **User Isolation**: Implement proper multi-user workspace separation  
- **Resource Limits**: Add CPU/memory limits for OpenCode processes
- **Monitoring**: Add metrics for session usage and performance
- **Reconnection**: Handle WebSocket disconnections gracefully

#### 4. Production Readiness
- **Origin Checking**: Remove `CheckOrigin: true` for WebSocket upgrader
- **Rate Limiting**: Add session creation rate limits per user
- **Logging**: Replace fmt.Printf with proper log package usage
- **Health Checks**: Add endpoint to monitor OpenCode session health
- **Graceful Shutdown**: Ensure all PTY processes terminate on server shutdown

### Technical Decisions Made

1. **PTY over Container**: Chose direct PTY approach over Dagger containers for simplicity and performance
2. **WebSocket over HTTP**: Real-time bidirectional communication required for terminal
3. **In-Memory Sessions**: Acceptable for prototype, but needs persistence for production
4. **Dynamic Imports**: xterm.js loaded on-demand to reduce main bundle size
5. **Port Range**: 30000-30099 provides 100 concurrent sessions

### User Experience Flow

1. User clicks pulsating red "Code" button in Station WebUI
2. Frontend calls `/api/v1/code/start` API with workspace path
3. Backend creates session, extracts OpenCode binary, starts PTY process
4. Frontend polls `/api/v1/code/session/{id}` until status becomes "running"
5. Frontend connects to WebSocket at returned URL
6. xterm.js terminal appears with direct connection to OpenCode TUI
7. User interacts with OpenCode normally through terminal
8. Ctrl+C or session timeout triggers cleanup

### Branch Information
- **Branch**: `feature/webui-opencode-integration`
- **Safe to Commit**: Yes, all changes are isolated and non-breaking
- **Ready for PR**: Needs testing first, then can merge to develop branch

### Environment Setup Required
- `ANTHROPIC_API_KEY` - For OpenCode AI functionality
- `OPENAI_API_KEY` - For OpenCode AI functionality  
- Station built with: `make local-install-ui-opencode`

---
*Last updated: 2025-08-21 - PTY + WebSocket bridge implementation complete*
*Next: End-to-end testing and user validation*