# PRD: Anthropic OAuth Provider for Claude Max/Pro

## Overview

Enable Station CLI users with Claude Max/Pro subscriptions to use their OAuth tokens instead of requiring a separate Anthropic API key. This brings parity with how OpenCode handles Anthropic OAuth authentication.

## Status: ✅ IMPLEMENTED

**Implementation Date**: 2025-12-30

The Anthropic OAuth provider is fully functional with the following capabilities:
- Full agent execution with MCP tools
- Multi-turn conversations with tool use
- Automatic token refresh before expiry
- Faker MCP servers with fallback to OpenAI

## Implementation Summary

### Architecture

We implemented a **native Anthropic OAuth plugin** that uses the `anthropic-sdk-go` directly, bypassing GenKit's compat_oai layer which caused issues with OAuth middleware.

| Component | Location | Purpose |
|-----------|----------|---------|
| Plugin | `internal/genkit/anthropic_oauth/plugin.go` | GenKit plugin initialization, client creation with OAuth headers |
| Generator | `internal/genkit/anthropic_oauth/generator.go` | API calls, streaming, tool support, response building |
| Config | `internal/config/config.go` | Token storage, automatic refresh logic |

### Key Implementation Details

1. **OAuth Headers**: Uses `Authorization: Bearer <token>` instead of `x-api-key`
2. **Beta Flags**: Includes required `anthropic-beta: oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14`
3. **Claude Code System Prompt**: Automatically injected as first system block (required for OAuth tokens)
4. **GenKit History() Fix**: Properly sets `Request` field in `ModelResponse` for conversation history tracking

### Files Modified

| File | Changes |
|------|---------|
| `internal/genkit/anthropic_oauth/plugin.go` | New - OAuth plugin with proper headers |
| `internal/genkit/anthropic_oauth/generator.go` | New - API calls, streaming, tool support |
| `internal/services/genkit_provider.go` | Integration with new OAuth plugin |
| `pkg/faker/mcp_faker.go` | Fallback to OpenAI when Anthropic configured |
| `pkg/faker/ai/client.go` | Fallback logic for child processes |
| `pkg/dotprompt/genkit_executor.go` | Nil response guard |

## Configuration

```yaml
# ~/.config/station/config.yaml
ai_provider: anthropic
ai_model: claude-opus-4-5-20251101  # or claude-sonnet-4-20250514, etc.
ai_auth_type: oauth
ai_oauth_token: sk-ant-oat01-...
ai_oauth_refresh_token: sk-ant-ort01-...
ai_oauth_expires_at: 1767135582033
```

## Test Results (2025-12-30)

### ✅ Simple Agent (No Tools)
```bash
stn agent run oauth-test-agent "Say hi like a pirate"
# Result: Full pirate response, 168 output tokens, ~7s
```

### ✅ Agent with Tools (k8s-operator)
```bash
stn agent run k8s-operator "List all deployments in the default namespace"
# Result: Called __list_deployments tool, returned structured JSON, ~35s
```

### ✅ Complex Multi-Tool Agent (incident-commander)
```bash
stn agent run incident-commander "P1 incident - high latency on payment service"
# Result: Called 6 tools, 4 API turns, structured incident response, ~97s
# Tools: __on_call_schedule, __service_health_check, __active_alerts, 
#        __get_pod_status, __incident_creation, __incident_escalation
```

### ✅ Faker MCP Fallback
```bash
stn faker --standalone --faker-id test-faker
# Result: Falls back to OpenAI gpt-4o-mini when Anthropic OAuth configured
```

### ✅ Token Refresh
- Automatic refresh when token expires within 5 minutes
- Refresh token stored alongside access token
- New tokens saved to config automatically

## Token Refresh Flow

```
getAIOAuthToken()
    ↓
getProviderAuthInfo("anthropic")
    ↓
getConfigOAuthCredentials()
    ↓
[Check if token expires within 5 min]
    ↓ (if expiring)
refreshClaudeToken(refreshToken)
    ↓
SaveOAuthTokens(newAccess, newRefresh, newExpiry)
```

## Supported Models

All Claude models registered in the OAuth plugin:
- `claude-opus-4-5-20251101` (Claude 4.5 Opus)
- `claude-opus-4-20250514` (Claude 4 Opus)
- `claude-sonnet-4-5-20250929` (Claude 4.5 Sonnet)
- `claude-sonnet-4-20250514` (Claude 4 Sonnet)
- `claude-haiku-4-5-20251001` (Claude 4.5 Haiku)
- `claude-3-5-sonnet-20241022` (Claude 3.5 Sonnet)
- `claude-3-5-haiku-20241022` (Claude 3.5 Haiku)
- `claude-3-opus-20240229` (Claude 3 Opus)

## Authentication Flow

```bash
# 1. Login (opens browser for OAuth)
stn auth anthropic login

# 2. Check status
stn auth anthropic status
# Output: Token expires: 2025-12-30T16:59:42-06:00 (5h51m0s remaining)

# 3. Logout
stn auth anthropic logout
```

## Known Limitations

1. **Claude Code System Prompt Required**: OAuth tokens are only authorized for Claude Code, so the system prompt must be present
2. **Faker Fallback**: Faker MCP servers cannot use Anthropic OAuth (falls back to OpenAI/Gemini)
3. **Token Expiry**: Tokens expire after ~4 hours, but automatic refresh handles this

## Success Criteria: ✅ ALL MET

- [x] `stn agent run <id> "task"` works with Anthropic OAuth
- [x] Performance comparable to OpenAI provider (~15s for simple task)
- [x] OAuth token refresh works automatically
- [x] Graceful fallback if OAuth fails (faker uses OpenAI)
- [x] Multi-turn tool use works correctly
- [x] Structured output (JSON schemas) work correctly

## Historical Context

### Previous Approach (Failed)

The initial implementation used `option.WithMiddleware()` to inject OAuth headers into the GenKit Anthropic plugin. This caused MCP tool discovery to hang indefinitely because:

1. GenKit's compat_oai layer has complex initialization
2. The middleware conflicted with internal HTTP client handling
3. MCP subprocess inheritance issues with OAuth config

### Solution

Created a native Anthropic OAuth plugin (`internal/genkit/anthropic_oauth/`) that:
1. Uses `anthropic-sdk-go` directly
2. Sets OAuth headers via `option.WithHeader()`
3. Injects Claude Code system prompt in the generator
4. Bypasses compat_oai layer entirely

This follows a similar pattern to OpenCode's implementation but adapted for GenKit integration.
