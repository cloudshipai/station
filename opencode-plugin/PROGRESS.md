# OpenCode Plugin - Development Progress

## Current Status: ALL TESTS PASSING ✅

**Date**: Jan 2, 2026

### Test Results

| Test | Result | Notes |
|------|--------|-------|
| Test 1: Git clone | ✅ PASS | Branch: master, Commit: 7fd1a60b |
| Test 2: Workspace reuse | ✅ PASS | Git pull + session continuation |
| Test 3: Branch checkout | ✅ PASS | Correct branch: test |
| Test 4: Non-git workspace | ✅ PASS | Instant session creation |

---

## Issues Fixed

### Issue 1: Docker BuildKit Cache (RESOLVED)

**Symptom**: Container loading `cartograph-plugin.js` instead of `station-plugin.js`

**Root Cause**: Docker BuildKit was caching layers from a previous project (Cartograph plugin) even with `--no-cache` flag.

**Fix**:
```bash
docker compose down -v
docker rmi test-opencode --force
docker builder prune -f
DOCKER_BUILDKIT=0 docker build -f test/Dockerfile.opencode -t test-opencode .
```

**Status**: RESOLVED - Plugin now loads correctly

---

## Current Hypothesis

### Issue 2: session.get Requires Directory Parameter for Git Repos

**Symptom**: After `session.create` succeeds, polling `session.get` never finds the session for git workspaces.

**Discovery**:
```bash
# Non-git workspace - works without directory (uses projectID: "global")
curl "http://localhost:4097/session/{id}"  # ✅ Works

# Git workspace - FAILS without directory (uses projectID: hash)
curl "http://localhost:4097/session/{id}"  # ❌ NotFoundError

# Git workspace - WORKS with directory
curl "http://localhost:4097/session/{id}?directory=/workspaces/git-test-..."  # ✅ Works
```

**Root Cause Analysis**:
- OpenCode stores sessions in project-specific directories
- Non-git workspaces use `projectID: "global"` → stored in `storage/session/global/`
- Git workspaces use `projectID: <hash>` → stored in `storage/session/<hash>/`
- `session.get` without `directory` param defaults to "global" storage
- Our `verifySessionExists()` wasn't passing the directory parameter

**Fix Applied**:
```typescript
// Before (broken for git repos)
private async verifySessionExists(opencodeID: string): Promise<boolean> {
  const result = await this.client.session.get({ path: { id: opencodeID } });
  return !result.error && !!result.data;
}

// After (works for all repos)
private async verifySessionExists(opencodeID: string, workspacePath: string): Promise<boolean> {
  const result = await this.client.session.get({ 
    path: { id: opencodeID },
    query: { directory: workspacePath }
  });
  return !result.error && !!result.data;
}
```

**Status**: ✅ RESOLVED

### Issue 3: session.prompt Also Needs Directory (RESOLVED)

After fixing `session.get`, the session was created successfully but `session.prompt` failed with "Session not found".

**Fix Applied** in `src/nats/handler.ts`:
```typescript
// Before (broken for git repos)
const promptOptions = {
  path: { id: sessionID },
  body: { parts: [...] },
};

// After (works for all repos)
const promptOptions = {
  path: { id: sessionID },
  query: { directory: workspacePath },
  body: { parts: [...] },
};
```

**Status**: ✅ RESOLVED

### Issue 4: Bun Shell Output Extraction (RESOLVED)

The git branch was showing as `[object Object]` because Bun shell returns a `ShellOutput` object, not a string.

**Fix Applied** in `src/workspace/manager.ts`:
```typescript
// Before (broken)
const branchResult = await this.shell`git branch --show-current`;
return { branch: String(branchResult).trim() };

// After (works)
const branchResult = await this.shell`git branch --show-current`;
return { branch: branchResult.text().trim() };
```

**Status**: ✅ RESOLVED

---

## Test Commands Quick Reference

```bash
# Rebuild everything
cd /home/epuerta/sandbox/cloudship-sandbox/station/opencode-plugin
bun run build
cd test && docker compose down -v
DOCKER_BUILDKIT=0 docker build -f Dockerfile.opencode -t test-opencode ..
docker compose up -d
sleep 5
NATS_URL=nats://localhost:4222 OPENCODE_URL=http://localhost:4097 bun run git-workspace-test.ts
```

---

## OpenCode API Findings

### Session Storage Behavior

| Workspace Type | projectID | Storage Path | session.get without dir |
|---------------|-----------|--------------|------------------------|
| Non-git | `"global"` | `storage/session/global/{id}.json` | Works |
| Git repo | `<hash>` | `storage/session/<hash>/{id}.json` | NotFoundError |

### Required: Always pass directory to session APIs for git workspaces
