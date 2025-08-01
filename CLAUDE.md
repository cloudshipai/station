# Claude Agent Context for Station Development

## Project Overview
Station is a secure, self-hosted platform for creating intelligent multi-environment MCP agents that integrate with Claude. The system allows users to build AI agents that understand development workflows, manage sensitive tools across environments, and run automated background tasks.

## Current System State

### Known Issues
- **SSH/MCP Shutdown Performance**: Graceful shutdown takes ~1m25s (should be <10s)
  - Likely causes: hanging MCP connections, database locks, resource cleanup delays
  - Needs investigation of timeout settings and connection pooling

- **CLI Agent Execution**: ✅ Implemented with clean server/fallback architecture
  - **Server Mode**: Uses `POST /api/v1/agents/:id/queue` with ExecutionQueueService for full execution
  - **Fallback Mode**: Provides simplified execution when server not available
  - **Clean DRY**: Reuses existing execution architecture, no code duplication
  - **Webhook Support**: Full webhook notifications work in server mode
  - **Graceful Degradation**: Clear messaging about mode limitations

### Active Agents
- **Home Directory Scanner** (ID: 2): Scheduled daily at midnight to scan home directory structure
  - Tools: list_directory, directory_tree, read_text_file, get_file_info, search_files
  - Max steps: 5

## Architecture Components

### Core Services
- **MCP Server**: Handles tool discovery and agent communication
- **Agent Management**: Scheduling, execution, and monitoring system  
- **Environment Management**: Multi-environment tool isolation (dev/staging/prod)
- **Security Layer**: Encryption, audit logging, access controls

### Key Directories
- `/station/` - Main project directory
- Config files likely in standard locations (`.config/`, `~/.station/`, etc.)
- Database: SQLite-based configuration storage

## Development Best Practices

### File Management
- **ALWAYS** prefer editing existing files over creating new ones
- **NEVER** create documentation files unless explicitly requested
- Use `TodoWrite` tool for task tracking and planning
- Maintain concise responses (≤4 lines unless detail requested)

### Security Guidelines
- Only assist with defensive security tasks
- Never create/modify code for malicious purposes
- Follow security best practices for secrets/keys
- Maintain environment isolation

### Code Conventions
- Analyze existing code patterns before making changes
- Check for available libraries/frameworks in codebase
- Follow existing naming conventions and typing patterns
- Never add comments unless requested

## Tool Usage Strategy

### Search and Discovery
- Use `Task` tool for open-ended searches requiring multiple rounds
- Use `Glob` for specific file pattern matching
- Use `Grep` for content searches with regex support
- Batch multiple tool calls when possible for performance

### MCP Integration
- Available MCP tools: file operations, directory operations, search & info
- Use `mcp__station__*` tools for agent management
- Config discovery via `mcp__station__discover_tools`

## Next Steps for New Agents

### Immediate Investigations Needed
1. **Shutdown Performance**: 
   - Find MCP config files with timeout settings
   - Analyze connection pooling configuration
   - Check database connection cleanup
   - Review graceful shutdown implementation

2. **System Monitoring**:
   - Set up performance monitoring for MCP operations
   - Add logging for connection lifecycle
   - Implement health checks for long-running processes

### Future Enhancements
- Improve agent scheduling flexibility
- Enhanced environment isolation features
- Better debugging tools for MCP operations
- Performance optimization for large-scale deployments

## Reference Documentation
- Main README: `/station/README.md` - Project overview and quick start
- Consider creating:
  - `TROUBLESHOOTING.md` - Common issues and solutions
  - `DEVELOPMENT.md` - Development setup and contribution guidelines
  - `API.md` - MCP API documentation and tool references

## Key Commands
```bash
stn init          # Initialize station in project
stn load <url>    # Load MCP server from GitHub
stn agent create  # Create new agent
stn status        # Check system status
```

## Context for New Chats
When starting a new conversation about Station:
1. Reference this file for current state
2. Check active agents and their status
3. Review known issues before proposing solutions
4. Maintain security-first approach for all implementations
5. Use TodoWrite for task planning and tracking

---
*Last updated: 2025-07-31 by Claude Agent*
*Key focus: SSH shutdown performance issue needs investigation*