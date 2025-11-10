# Faker Configuration Learnings

## Session Summary: Tool Cache and Model Configuration

**Date**: 2025-11-10  
**Focus**: Understanding faker tool caching, model configuration, and PATH issues

## Key Questions Answered

### 1. Where are the faker configs stored?

**Answer**: Faker tool schemas are cached in the `faker_tool_cache` database table.

**Schema**:
```sql
CREATE TABLE faker_tool_cache (
    id INTEGER PRIMARY KEY,
    faker_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    tool_schema TEXT NOT NULL,  -- Full OpenAPI JSON schema
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)
```

**Key Insight**: The `tool_schema` column contains the COMPLETE OpenAPI-style JSON schema for each tool, not just the tool name. This includes:
- Tool description
- Parameters with types and descriptions
- Required fields
- Input/output schemas

### 2. Will re-running generate the same tools?

**Answer**: **NO** - Tools are AI-generated and will vary between runs.

**Evidence**:
- First sync: aws-cost-faker generated 5 tools
- After cache clear: Sync would generate different tools (potentially different names/counts)
- AI interprets the `--ai-instruction` text and generates tools based on LLM output

**Implication**: Tool caching is CRITICAL for consistency. Once tools are generated and agents are configured to use them, the cache should be preserved.

### 3. Can we configure AI model/provider for fakers?

**Answer**: **YES** - Use `--ai-model` flag in faker configuration.

**Updated Configuration**:
```json
{
  "mcpServers": {
    "aws-cost-faker": {
      "command": "/path/to/stn",
      "args": [
        "faker",
        "--standalone",
        "--faker-id", "aws-cost-faker",
        "--ai-model", "gpt-4o-mini",  // NEW: Model selection
        "--ai-instruction", "...",
        "--debug"
      ]
    }
  }
}
```

**Available Flags**:
- `--ai-model`: Overrides Station's configured model (e.g., `gpt-4o-mini`, `gpt-4o`, `gemini-1.5-flash`)
- ~~`--ai-provider`~~: NOT SUPPORTED - uses Station's configured provider from `config.yaml`

**Provider Configuration**: Set in `/home/epuerta/.config/station/config.yaml`:
```yaml
ai_provider: openai
ai_model: gpt-4o-mini
```

### 4. Can we use `stn` from PATH instead of full path?

**Answer**: **NO** - Using `stn` from PATH causes MCP server hangs.

**Issue Discovered**:
- PATH version: `/home/epuerta/.local/bin/stn` (140MB)
- Project version: `/home/epuerta/projects/hack/station/bin/stn` (72MB)
- Both report same version (`v0.1.0`) but behave differently
- Using PATH version causes pooled MCP server to hang at startup

**Current Workaround**: Use full path to project's `stn` binary in faker configurations.

**Symptoms of PATH Issue**:
```
Starting pooled MCP server: aws-cost-faker:...
[hangs indefinitely, no error message]
```

## Critical Learnings

### Tool Cache Architecture

**Cache Behavior**:
1. **First Sync**: Faker generates tools via AI, stores schemas in `faker_tool_cache`
2. **Subsequent Syncs**: Tools are read from cache (no AI calls)
3. **Cache Clear**: Next sync regenerates tools (potentially different tools!)

**Cache Location**: SQLite database at `/home/epuerta/.config/station/station.db`

**Viewing Cache**:
```sql
SELECT faker_id, tool_name, 
       json_extract(tool_schema, '$.description') as description
FROM faker_tool_cache 
WHERE faker_id='aws-cost-faker';
```

### Tool Schema Requirements

**OpenAI requires proper JSON schema**:
```json
{
  "name": "get_cost_and_usage",
  "description": "Query AWS Cost Explorer for cost and usage data",
  "parameters": {
    "type": "object",
    "properties": {
      "time_period": {
        "type": "object",
        "description": "Start and end dates for the query"
      },
      "granularity": {
        "type": "string",
        "enum": ["DAILY", "MONTHLY"]
      }
    }
  }
}
```

**Empty schemas cause errors**:
```
400 Bad Request: Invalid schema for function '__get_anomalies': 
'' is not valid under any of the given schemas.
```

### Cache Management Best Practices

**DO**:
- ✅ Preserve cache once agents are configured and working
- ✅ Back up cache before experimental changes
- ✅ Let sync complete fully to generate proper schemas
- ✅ Use `--debug` flag to see tool generation process

**DON'T**:
- ❌ Clear cache unless you want different tools
- ❌ Manually insert tools (schemas will be incomplete)
- ❌ Expect consistent tool names across cache clears
- ❌ Interrupt sync during tool generation

### Model Selection Strategy

**Cost-Effective Fakers**:
```json
"--ai-model", "gpt-4o-mini"  // $0.150 per 1M input tokens
```

**High-Quality Fakers**:
```json
"--ai-model", "gpt-4o"  // More expensive but better schemas
```

**Fast Fakers** (if using Gemini):
```json
"--ai-model", "gemini-1.5-flash"  // Fastest, cheapest
```

## Troubleshooting Guide

### Issue: Agent fails with "Invalid schema for function"

**Cause**: Tool cache contains incomplete schemas (manually inserted or corrupted)

**Solution**:
1. Clear cache for that faker: `DELETE FROM faker_tool_cache WHERE faker_id='...'`
2. Run full sync: `stn sync <environment>`
3. Wait for complete tool generation (may take 60-120s)
4. Verify schemas: `SELECT tool_schema FROM faker_tool_cache WHERE faker_id='...'`

### Issue: MCP server hangs at startup

**Cause**: Using `stn` from PATH instead of full path

**Solution**: Use full path in `template.json`:
```json
"command": "/home/epuerta/projects/hack/station/bin/stn"
```

### Issue: Different tools generated after cache clear

**Cause**: AI generates tools non-deterministically

**Solution**:
1. Accept that tools will differ
2. Update agent `.prompt` files to match new tool names
3. OR restore cache from backup before clear

### Issue: Sync takes 60+ seconds

**Cause**: Tool generation requires multiple OpenAI API calls per faker

**Expected Behavior**: 
- 3 fakers × 5-8 tools each = 15-24 tool generation calls
- Each call: 2-5 seconds
- Total: 30-120 seconds for cold start
- Subsequent syncs: <5 seconds (uses cache)

## Recommendations

### For Production

1. **Freeze Tool Cache**: Once agents work, don't clear cache
2. **Version Control**: Export tool schemas to JSON for backup
3. **Model Selection**: Use `gpt-4o-mini` for cost-effective fakers
4. **Path Configuration**: Always use full path to `stn` binary

### For Development

1. **Experiment Safely**: Test with separate environment/cache
2. **Schema Validation**: Check `tool_schema` column after generation
3. **Sync Monitoring**: Use `--debug` to watch tool generation
4. **Cache Backups**: SQLite backup before cache clears

## Future Improvements

### Should Implement

1. **Response Caching**: Cache AI-generated responses by parameters
2. **Schema Validation**: Validate tool schemas before storing
3. **PATH Resolution**: Fix PATH vs full path issue
4. **Deterministic Tools**: Add seed/hash to generate consistent tools

### Nice to Have

1. **Tool Export/Import**: CLI commands to backup/restore tools
2. **Schema Editor**: UI to view/edit tool schemas
3. **Cache Stats**: Show cache hit rates, generation times
4. **Model Performance**: Track token usage per model type

## Related Files

- `~/.config/station/station.db` - Tool cache database
- `~/.config/station/environments/*/template.json` - Faker configurations
- `migrations/016_add_faker_tool_cache.sql` - Cache table schema
- `pkg/faker/tool_generator.go` - Tool generation logic
- `pkg/faker/toolcache/cache.go` - Cache management

---

**Created**: 2025-11-10  
**Context**: Complex faker workflow investigation  
**Key Finding**: Tool caching is critical for consistency - cache schemas, not just names
