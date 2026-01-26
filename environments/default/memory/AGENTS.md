# Project Context

## Overview
This is a DevOps automation platform built with Go and React.

## Build & Test Commands
- `make build` - Build all components
- `make test` - Run unit tests
- `make e2e` - Run E2E tests with real LLM

## Architecture Notes
- Backend: Go with Genkit for LLM orchestration
- Frontend: React with TypeScript
- Database: PostgreSQL for runs, NATS KV for sessions

## Code Style
- Use `slog` for structured logging
- Error wrapping with `fmt.Errorf("context: %w", err)`
- Table-driven tests preferred

## User Preferences
- Prefers concise explanations
- Uses dark mode
- Timezone: PST

## Learned Patterns
- When querying Prometheus, always use the `prometheus-query` MCP tool
- For Kubernetes issues, check pod events first via `kubectl describe`
- User prefers markdown tables over bullet lists for comparison data
