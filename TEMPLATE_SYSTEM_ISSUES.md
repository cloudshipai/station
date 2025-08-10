# Template System Issues - Testing Report

**Date**: 2025-08-10  
**Context**: Comprehensive multi-environment testing of template bundle workflow  
**Branch**: `fix/template-system-issues`

## 🎯 Testing Scope

- Clean station install with fresh database
- Multi-environment setup (default, dev, staging, prod)
- Template variable resolution across environments
- MCP server configuration and tool discovery
- Agent deployment and tool assignment
- Cross-environment isolation

## ❌ Critical Issues Identified

### Issue #1: Missing variables.yml Causes Hard Failure
**Severity**: High  
**Component**: Template Engine / UX  
**Status**: 🔴 Blocking

**Problem**:
```bash
❌ mcp_grafana_config: failed to process template variables: failed to load environment variables: failed to read variables file: open /home/epuerta/.config/station/environments/staging/variables.yml: no such file or directory
```

**Expected Behavior**: System should gracefully handle missing variables.yml files
**Current Behavior**: Hard failure with cryptic error message

**Solutions**:
- [ ] Auto-create empty variables.yml when environment is created
- [ ] Gracefully handle missing variables files (treat as empty)
- [ ] Better error messages guiding users to create variables.yml
- [ ] Template validation to warn about missing variables

---

### Issue #2: Template Variables Not Actually Resolved
**Severity**: Critical  
**Component**: Go Template Engine Integration  
**Status**: 🔴 Blocking

**Problem**:
Template variables remain unresolved in config files after "successful" sync:
```json
{
  "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .FILESYSTEM_ROOT }}"],
}
```

**Expected**: Variables should be resolved to actual values like `/home/epuerta/dev`
**Root Cause**: Template engine processes content in memory but doesn't persist resolved content

**Investigation Needed**:
- [ ] Verify template engine is actually being called
- [ ] Check if resolved content is being stored vs original template content  
- [ ] Confirm variable loading from variables.yml is working
- [ ] Add debug logging to trace template resolution pipeline

---

### Issue #3: MCP Tools Not Discovered After Sync
**Severity**: High  
**Component**: Tool Discovery / MCP Integration  
**Status**: 🔴 Blocking

**Problem**:
```bash
🏠 Listing local tools
• No tools found
```

Even after successful sync with "Error reading response: read |0: file already closed" warning.

**Expected**: AWS and filesystem tools should be discovered and listed  
**Potential Causes**:
- Template variables not resolved → MCP servers can't connect
- MCP server startup failures
- Tool discovery timeout issues
- Database storage problems

**Dependencies**: Likely blocked by Issue #2 (unresolved template variables)

---

### Issue #4: Config Deletion Removes Source Files
**Severity**: Medium  
**Component**: Configuration Management  
**Status**: 🟡 UX Problem

**Problem**:
```bash
./bin/stn mcp delete aws --environment dev --confirm
# Result: Deletes /home/epuerta/.config/station/environments/dev/aws.json from disk
```

**Expected**: Should only remove database records, preserve config files  
**Impact**: Users lose their configuration files when trying to force re-sync

**Solutions**:
- [ ] Separate `delete` (remove DB record) from `remove` (delete files)
- [ ] Add `--delete-files` flag for explicit file deletion
- [ ] Default behavior should preserve files

---

### Issue #5: Poor Error Messages and User Guidance
**Severity**: Low  
**Component**: User Experience  
**Status**: 🟡 Enhancement

**Examples**:
- Cryptic template engine errors
- No guidance on creating variables.yml
- No clear indication of what template variables are expected
- Silent failures in tool discovery

**Solutions**:
- [ ] Better error messages with actionable guidance
- [ ] Template validation with variable requirements
- [ ] Clear success/failure indicators
- [ ] Documentation improvements

## ✅ What's Working

- ✅ Multi-environment creation and isolation
- ✅ Config file deployment across environments
- ✅ Agent creation and database storage
- ✅ Template bundle installation workflow
- ✅ Basic sync operations (when variables exist)
- ✅ Environment-specific configuration loading

## 🔧 Fix Priority Order

1. **Issue #2** (Template resolution) - Most critical, likely root cause of #3
2. **Issue #1** (Missing variables handling) - Blocks user adoption
3. **Issue #3** (Tool discovery) - May be resolved by fixing #2
4. **Issue #4** (Config deletion behavior) - UX improvement
5. **Issue #5** (Error messages) - Polish

## 🧪 Test Cases to Validate Fixes

### Template Resolution Test
```bash
# Setup
echo "TEST_VAR: test-value" > /env/variables.yml
echo '{"test": "{{ .TEST_VAR }}"}' > /env/config.json

# Expected result after sync
cat /env/config.json # Should show: {"test": "test-value"}
```

### Missing Variables Test  
```bash
# Setup: No variables.yml file
echo '{"test": "no-vars"}' > /env/config.json

# Expected: Should sync successfully without errors
./bin/stn sync env
```

### Tool Discovery Test
```bash
# After template resolution fix
./bin/stn mcp tools dev
# Expected: Should show AWS and filesystem tools
```

## 📋 Definition of Done

- [ ] All template variables resolve correctly in config files
- [ ] Missing variables.yml handled gracefully
- [ ] MCP tools discovered and listed after sync
- [ ] Config deletion preserves source files by default
- [ ] Clear, actionable error messages
- [ ] All test cases pass
- [ ] No regressions in existing functionality

---

**Note**: This is a comprehensive testing day to identify and fix all template system issues systematically.