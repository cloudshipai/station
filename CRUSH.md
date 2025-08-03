# CRUSH.md

This file documents the build, lint, and test commands, along with code style rules, for agents operating in this multi-project repository. It is curated for agents like Crush, Copilot, etc. **Update this file if conventions change.**

## General
- Multi-project monorepo: TypeScript (with Bun/yarn/pnpm), Go, Astro (web), etc.
- Set up project-specific environments before running commands (see each package's README or CONTRIBUTING.md).

---

## Build, Lint, & Test

**TypeScript/Bun/Opencode agent core:**
- install: `bun install`
- build: *N/A* (Bun auto-bundles)
- typecheck: `bun run typecheck` or `npm run typecheck`
- lint: see `.editorconfig`, often `bun lint` or `yarn lint` or `npm run lint`
- test (all): `bun test`
- test (single file): `bun test test/tool/tool.test.ts`

**Go modules:**
- build: `go build -o <BIN> cmd/main.go`
- test (all): `go test ./...`
- test (single file): `go test -v ./path/to/file_test.go`
- lint: `golangci-lint run` or `gofmt -l .`

**SDK (TypeScript, yarn):**
- install: `yarn`
- build: `yarn build`
- test: `yarn test`
- lint: `yarn lint`

**TUI SDK (Go):**
- setup/build: `./scripts/bootstrap` `./scripts/build`
- test: `./scripts/test`

**Web/Astro (npm or pnpm):**
- build: `npm run build` or `pnpm build`
- test: (rare; see README)

---

## Code Style Guidelines

- **Imports:** Prefer named/relative imports. Use ESM for TS, standard imports for Go.
- **Formatting:** Use Prettier (TS), `.editorconfig` (all), `gofmt` (Go).
- **Types/Naming:**
    - TypeScript: camelCase for functions/vars, PascalCase for types/classes, avoid `any`, prefer Zod for validation.
    - Go: camelCase for vars, PascalCase for exported, snake_case for files when idiomatic.
- **Error Handling:**
    - TypeScript: avoid exceptions, prefer Result patterns, use Zod for input validation.
    - Go: always handle errors, never ignore `err`, return early.
- **Other:**
    - Minimize use of destructuring and `else` blocks.
    - Prefer expressive, single-word var names.
    - Use Bun APIs where possible (`Bun.file()`).
    - See `.editorconfig` for whitespace, indent, newline style.

---

## Tooling & Architecture
- Each tool implements a `Tool.Info` interface with an `execute()` method (TS ops).
- Always validate inputs with Zod (TS).
- Use dependency injection (DI) via `App.provide()` where possible (TS).
- Go TUI communicates with server via Stainless SDK—update client SDK if server endpoints change.

---
- No proprietary Copilot/Cursor instructions detected (add here if found).
- Add `.crush` directory to `.gitignore` if present.