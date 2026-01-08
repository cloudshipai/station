# Station Lattice Architecture Comparison

A comparative analysis of Station Lattice against wasmCloud and Gastown architectures, with recommendations for improvements.

## Architecture Comparison Matrix

| Aspect | wasmCloud | Gastown | Station Lattice |
|--------|-----------|---------|-----------------|
| **Language** | Rust | Go | Go |
| **Transport** | NATS | Git + tmux | NATS |
| **State Persistence** | JetStream KV | Git (Beads) | JetStream KV |
| **Work Units** | WebAssembly components | AI agents (Claude) | AI agents (any LLM) |
| **Discovery** | Auction-based bidding | Beads queries | Registry scan |
| **Routing** | Capability contracts | Address-based | Capability-based |
| **Security** | Cryptographic identities | File-based | Token/user auth |
| **Lifecycle** | Host-managed | Witness/Deacon | Presence heartbeat |
| **Crash Recovery** | JetStream replay | Git persistence | JetStream KV |
| **Multi-tenancy** | Lattice prefix isolation | Town/Rig separation | Single namespace |

## NATS Subject Patterns Comparison

### wasmCloud
```
wasmbus.ctl.{version}.{lattice}.{resource}.{action}[.{target}]
wasmbus.ctl.v1.default.component.scale.host123
wasmbus.ctl.v1.default.host.ping
wasmbus.ctl.v1.default.link.put

wasmbus.rpc.{lattice}.{provider}.{link}.{operation}
wasmbus.evt.{lattice}.{event_type}
```

**Strengths:**
- Version prefix enables protocol evolution
- Lattice isolation built-in
- Clear resource/action hierarchy

### Gastown
```
# No NATS - uses git-backed Beads with addresses
{rig}/{role}                    # gastown/witness
{rig}/{type}/{name}             # gastown/polecats/Toast
@{group}                        # @witnesses, @rig/gastown
```

**Strengths:**
- Simple, human-readable addressing
- Group addressing for broadcasts
- Hierarchical organization

### Station Lattice (Current)
```
lattice.presence.{action}                           # announce, heartbeat, goodbye
lattice.station.{station_id}.agent.invoke           # agent execution
lattice.station.{station_id}.workflow.run           # workflow execution
lattice.station.{station_id}.work.assign            # async work
lattice.work.{work_id}.response                     # work responses
```

**Weaknesses:**
- No version prefix
- No multi-tenancy isolation
- Inconsistent hierarchy (station vs work at same level)

## Recommended Subject Pattern (Improved)

```
# Version and namespace isolation
lattice.v1.{org_id}.{category}.{target}.{action}

# Presence (broadcast)
lattice.v1.{org_id}.presence.announce
lattice.v1.{org_id}.presence.heartbeat
lattice.v1.{org_id}.presence.goodbye

# Station Control (targeted)
lattice.v1.{org_id}.station.{station_id}.agent.invoke
lattice.v1.{org_id}.station.{station_id}.workflow.run
lattice.v1.{org_id}.station.{station_id}.work.assign

# Work Responses (targeted)
lattice.v1.{org_id}.work.{work_id}.progress
lattice.v1.{org_id}.work.{work_id}.complete
lattice.v1.{org_id}.work.{work_id}.failed

# Events (broadcast, high-volume)
lattice.v1.{org_id}.events.station.{event_type}
lattice.v1.{org_id}.events.agent.{event_type}
lattice.v1.{org_id}.events.work.{event_type}
```

## Key Patterns to Adopt

### 1. From wasmCloud: Auction-Based Scheduling

**Current:** First-available routing with local preference
**Proposed:** Hosts bid on work based on capacity/capabilities

```go
// AuctionRequest sent to all stations
type AuctionRequest struct {
    WorkID       string
    AgentName    string
    Capabilities []string
    Constraints  map[string]string
    Timeout      time.Duration
}

// AuctionBid from interested stations  
type AuctionBid struct {
    StationID    string
    Score        float64  // Capacity, locality, etc.
    Capabilities []string
    Load         float64
}
```

**Impact:** Better load distribution, smarter placement
**Effort:** Medium (new auction protocol)

### 2. From Gastown: Witness Lifecycle Pattern

**Current:** Heartbeat presence, no stuck-work detection
**Proposed:** Witness loop monitors work health

```go
type Witness struct {
    CheckInterval time.Duration  // 30s
    StuckThreshold time.Duration // 5min
    MaxRetries    int            // 3
}

// Witness loop
func (w *Witness) Monitor(ctx context.Context) {
    for {
        // Check all in-progress work
        stuckWork := w.FindStuckWork()
        for _, work := range stuckWork {
            if work.Retries < w.MaxRetries {
                w.RequeueWork(work)
            } else {
                w.EscalateWork(work)
            }
        }
        time.Sleep(w.CheckInterval)
    }
}
```

**Impact:** Better reliability, automatic recovery
**Effort:** Low (extend existing work system)

### 3. From wasmCloud: Event Streaming

**Current:** Request-reply only, limited observability
**Proposed:** Event stream for all state changes

```go
// CloudEvents format
type LatticeEvent struct {
    ID          string    `json:"id"`
    Source      string    `json:"source"`      // station ID
    Type        string    `json:"type"`        // station.joined, agent.invoked, etc.
    Time        time.Time `json:"time"`
    Data        any       `json:"data"`
    DataSchema  string    `json:"dataschema"`
}

// Event types
const (
    EventStationJoined    = "station.joined"
    EventStationLeft      = "station.left"
    EventAgentRegistered  = "agent.registered"
    EventAgentInvoked     = "agent.invoked"
    EventWorkAssigned     = "work.assigned"
    EventWorkCompleted    = "work.completed"
)
```

**Impact:** Better observability, audit trail, integrations
**Effort:** Medium (new event system)

### 4. From wasmCloud: Multi-Tenancy Isolation

**Current:** Single namespace, no isolation
**Proposed:** Org-based subject prefix isolation

```go
type LatticeConfig struct {
    OrgID        string  // CloudShip org ID
    SubjectPrefix string // auto-generated: lattice.v1.{org_id}
}

// Subject generation
func (c *LatticeConfig) Subject(parts ...string) string {
    return fmt.Sprintf("lattice.v1.%s.%s", c.OrgID, strings.Join(parts, "."))
}
```

**Impact:** Multi-tenant safety, CloudShip integration
**Effort:** Low (string prefix change)

### 5. From Gastown: Work Hooks (Propulsion)

**Current:** Work hook exists but no autonomous execution
**Proposed:** "Propulsion principle" - work on hook = immediate execution

```go
// On station startup
func (s *Station) OnStart(ctx context.Context) error {
    // Check for assigned work immediately
    pendingWork, err := s.workStore.GetPendingForStation(ctx, s.ID)
    if err != nil {
        return err
    }
    
    // Execute immediately (propulsion principle)
    for _, work := range pendingWork {
        go s.executeWork(ctx, work)
    }
    
    return nil
}
```

**Impact:** Better crash recovery, faster resumption
**Effort:** Low (already partially implemented)

## Improvement Roadmap

### Phase 1: Foundation (2-3 weeks)
1. **Multi-tenancy isolation** - Add org_id to subject patterns
2. **Witness pattern** - Implement stuck-work detection
3. **Propulsion on startup** - Resume pending work automatically

### Phase 2: Observability (2-3 weeks)
4. **Event streaming** - CloudEvents for all state changes
5. **Enhanced telemetry** - Metrics for work queue depth, latency
6. **Audit log** - JetStream append-only event stream

### Phase 3: Scalability (3-4 weeks)
7. **Auction-based scheduling** - Replace first-available routing
8. **Registry sharding** - Partition by capability/org
9. **HA orchestrator** - NATS clustering, leader election

### Phase 4: Security (2-3 weeks)
10. **Station identity** - Ed25519 keypairs for stations
11. **Message signing** - Sign work requests/responses
12. **Fine-grained ACLs** - Per-station NATS permissions

## Current Station Lattice Strengths

Things we're doing well (keep these):

1. **OpenTelemetry integration** - Better than both wasmCloud and Gastown
2. **Work progress streaming** - Real-time updates during execution
3. **Capability-based routing** - Flexible agent discovery
4. **JetStream persistence** - Solid foundation for state
5. **Embedded NATS option** - Easy single-node deployment
6. **CLI client mode** - Query lattice without running server

## Architecture Decision: What NOT to Adopt

### Git-backed persistence (Gastown)
- **Rationale:** JetStream KV is sufficient, git adds complexity
- **Alternative:** JetStream event stream for audit trail

### WebAssembly components (wasmCloud)
- **Rationale:** Our agents are LLM-based, not WASM modules
- **Alternative:** Keep Python/Go agents with natural language interfaces

### Cryptographic component identity (wasmCloud)
- **Rationale:** Over-engineered for our use case
- **Alternative:** Station-level auth with NATS tokens/NKeys

### Tmux session management (Gastown)
- **Rationale:** We're distributed, not local multi-agent
- **Alternative:** Keep NATS-based messaging

## Conclusion

Station Lattice has a solid foundation. The top three improvements by impact/effort ratio:

1. **Multi-tenancy isolation** (Low effort, High impact)
2. **Witness pattern for stuck work** (Low effort, High impact)
3. **Event streaming** (Medium effort, High impact)

These changes would bring Station Lattice closer to wasmCloud's robustness while maintaining its simplicity and LLM-agent focus.
