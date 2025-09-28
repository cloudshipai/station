# Station TODO - Future Features

## Redis Integration for Organization ID Lookup
**Why**: Currently using in-memory cache for organization ID lookup. Should implement Redis for persistent caching across restarts.

**Implementation Plan**:
1. Add Redis client to LighthouseClient struct
2. Implement cache pattern:
   - `HGET org_cache {registration_key}` (check cache)
   - If miss: Make gRPC call to resolve org ID from registration key
   - `HSET org_cache {registration_key} {org_id} EX 3600` (cache result)
3. Add Redis configuration to LighthouseConfig
4. Add graceful fallback when Redis is unavailable

**Priority**: Medium
**Estimated Effort**: 2-3 hours
**Dependencies**: Redis deployment in CloudShip infrastructure

---

*Last updated: 2025-09-28*