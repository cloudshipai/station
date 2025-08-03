# Agent Environment Specificity - Product Requirements Document

## Overview
Transform Station's agent system to be environment-specific, ensuring agents can only use tools from their designated environment. This eliminates cross-environment tool confusion and creates cleaner, more predictable agent behavior.

## Current State Problems
- Agents can access tools across multiple environments
- Tool selection is messy and confusing during agent creation
- No clear environment boundaries for agent execution
- Cross-environment tool conflicts possible
- Genkit agents can access tools from any environment
- Tool filtering and discovery spans environments inappropriately
- Agent execution context lacks environment isolation
- MCP tool namespacing not environment-aware

## Target State Goals
- Agents belong to exactly ONE environment
- Agent tools come exclusively from that environment's MCP configurations
- Clean agent creation flow within environment context
- Environment isolation for agent execution

## Implementation Checklist

### Phase 1: Analysis & Database Layer ✅❌⭕
- [ ] Analyze current database schema for agent-environment relationships
- [ ] Check git migration history for previous environment-specific implementation
- [ ] Design new schema changes needed
- [ ] Create database migrations
- [ ] Update repository layer for environment-specific queries
- [ ] Write Go tests for repository changes
- [ ] Verify database layer with integration tests

### Phase 2: Service Layer ✅❌⭕
- [ ] Update AgentService to enforce environment specificity
- [ ] Modify ToolDiscoveryService to be environment-scoped
- [ ] Update GenKitService for environment-specific tool access
- [ ] Refactor IntelligentAgentCreator for environment-bound tools
- [ ] Update tool filtering and namespacing logic
- [ ] Modify agent execution context for environment isolation
- [ ] Update MCP tool registration for environment specificity
- [ ] Write comprehensive unit tests for service layer
- [ ] Create integration tests for service interactions
- [ ] Verify service layer isolation

### Phase 3: CLI Layer ✅❌⭕
- [ ] Update `stn agent create` to require environment parameter
- [ ] Modify agent listing to show environment context
- [ ] Update agent execution commands for environment awareness
- [ ] Add environment validation to CLI workflows
- [ ] Write CLI integration tests
- [ ] Test CLI commands end-to-end
- [ ] Verify CLI parameter validation

### Phase 4: API Layer ✅❌⭕
- [ ] Update REST API endpoints for environment-scoped agents
- [ ] Modify agent creation API to enforce environment binding
- [ ] Update agent listing/filtering by environment
- [ ] Add environment validation to API handlers
- [ ] Write API integration tests
- [ ] Test API endpoints with Postman/curl
- [ ] Verify API response formats

### Phase 5: MCP Layer ✅❌⭕
- [ ] Update MCP server agent endpoints for environment specificity
- [ ] Modify tool discovery responses to be environment-scoped
- [ ] Update Genkit MCP plugin for environment-aware tool access
- [ ] Refactor tool namespacing for environment boundaries
- [ ] Update agent execution context for environment isolation
- [ ] Add environment metadata to MCP responses
- [ ] Update tool filtering in MCP server responses
- [ ] Write MCP integration tests
- [ ] Test MCP protocol compliance with environment isolation
- [ ] Verify MCP tool isolation and Genkit integration

### Phase 6: TUI Layer ✅❌⭕
- [ ] Update TUI agent creation screens for environment selection
- [ ] Modify agent listing views to show environment context
- [ ] Update tool selection screens to be environment-scoped
- [ ] Add environment indicators throughout TUI
- [ ] Test TUI workflows manually
- [ ] Verify TUI user experience flows
- [ ] Final TUI testing with user

## Testing Strategy

### Unit Tests (Go)
```bash
# Run specific test packages
go test ./internal/db/repositories/... -v
go test ./internal/services/... -v
go test ./cmd/main/handlers/... -v
```

### Integration Tests
```bash
# Database integration
go test ./tests/integration/db/... -v

# Service integration  
go test ./tests/integration/services/... -v

# API integration
go test ./tests/integration/api/... -v
```

### End-to-End CLI Tests
```bash
# Test agent creation with environment
stn agent create --environment dev --domain test

# Test agent listing by environment
stn agent list --environment dev

# Test agent execution in environment
stn agent run 1 --environment dev
```

### API Tests (curl/Postman)
```bash
# Test environment-scoped agent creation
curl -X POST /api/v1/agents -d '{"environment_id": 1, "name": "test"}'

# Test environment filtering
curl /api/v1/agents?environment=dev
```

### Manual TUI Tests
- Agent creation workflow with environment selection
- Tool selection shows only environment tools
- Agent execution respects environment boundaries
- Environment indicators visible throughout interface

## Success Criteria

### Functional Requirements
- ✅ Agents can only be created within a specific environment
- ✅ Agent tools come exclusively from that environment's MCP configs
- ✅ Agent execution is isolated to environment tools
- ✅ Clear environment context throughout all interfaces

### Technical Requirements
- ✅ Database schema enforces environment-agent relationships
- ✅ All API endpoints respect environment boundaries
- ✅ CLI commands require and validate environment parameters
- ✅ TUI clearly shows environment context
- ✅ Comprehensive test coverage at all layers

### User Experience Requirements
- ✅ Clear environment selection during agent creation
- ✅ Environment context visible in agent listings
- ✅ No confusion about which tools are available
- ✅ Consistent environment indicators across interfaces

## Risk Mitigation
- Database migration safety with rollback plan
- Backward compatibility for existing agents
- Comprehensive testing at each layer before proceeding
- Git commits after each successful phase

## Timeline
- Phase 1 (DB): 1-2 hours
- Phase 2 (Service): 2-3 hours  
- Phase 3 (CLI): 1-2 hours
- Phase 4 (API): 1-2 hours
- Phase 5 (MCP): 1-2 hours
- Phase 6 (TUI): 1 hour + user testing

**Total Estimated Time: 7-12 hours**

## Testing Feedback Loop
1. **Write failing test** → 2. **Implement feature** → 3. **Run test until green** → 4. **Commit** → 5. **Repeat**

Each phase follows this pattern with appropriate test types (unit/integration/e2e).