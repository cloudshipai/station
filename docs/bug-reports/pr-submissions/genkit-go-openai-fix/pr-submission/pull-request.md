# Fix critical tool_call_id bug in OpenAI plugin

## Summary

This PR fixes a critical bug in the Firebase Genkit Go OpenAI plugin where tool execution results were incorrectly used as `tool_call_id` values instead of the original OpenAI-generated reference IDs. This bug made **all multi-turn tool calling completely unusable** with OpenAI-compatible APIs.

## Problem

The OpenAI plugin was using `part.ToolRequest.Name` (which contains tool execution results) as the `tool_call_id` when responding to OpenAI. However, OpenAI requires the `tool_call_id` to match the original short identifier (e.g., `call_abc123`) from the initial tool call request.

### Impact
- ❌ **OpenAI API**: Complete failure with "tool_call_id not found" errors
- ❌ **Anthropic via OpenAI API**: Complete failure
- ❌ **Any OpenAI-compatible provider**: Complete failure  
- ❌ **MCP tools**: Failure with "string too long" errors (responses > 40 chars)
- ✅ **Google Gemini**: Works (different code path)

### Error Examples
```
Invalid parameter: 'tool_call_id' of '"Found documentation about S3 policies..."' not found in 'tool_calls' of previous message.
```

```
Expected a string with maximum length 40, but got a string with length 1721 instead.
```

## Root Cause

**File**: `/go/plugins/compat_oai/generate.go`  
**Line**: 402

```go
// BROKEN CODE:
ID: (part.ToolRequest.Name),  // Uses tool result as ID ❌
```

The comment in the code reveals the developers knew this was wrong:
```go
// NOTE: Temporarily set its name instead of its ref (i.e. call_xxxxx) 
// since it's not defined in the ai.ToolRequest struct.
```

## Solution

### Minimal Fix
Change line 402 from:
```go
ID: (part.ToolRequest.Name),     // Tool result as ID ❌
```

To:
```go  
ID: (part.ToolRequest.Ref),      // Original OpenAI ID ✅
```

### Complete Fix (Recommended)
- Use `Ref` for tool call IDs
- Add fallback ID generation when `Ref` is empty
- Ensure tool responses use matching IDs
- Add comprehensive test coverage

## Comparison with JavaScript Implementation

The JavaScript implementation correctly uses the `ref` field:

```javascript
// JavaScript (working):
id: part.toolRequest.ref ?? '',          // Uses ref ✅
tool_call_id: part.toolResponse.ref ?? '',  // Uses ref ✅
```

```go
// Go (broken):
ID: (part.ToolRequest.Name),              // Uses name ❌
```

## Testing

### Reproduction Cases
1. **Simple tools**: Short responses that still fail ID correlation
2. **MCP tools**: Complex JSON responses that exceed 40-character limit
3. **Multi-turn**: Consecutive tool calls that break after first turn

### Test Results After Fix
- ✅ Simple tool calls succeed
- ✅ Long tool responses work correctly  
- ✅ MCP tool integration works
- ✅ Multi-turn conversations flow properly

## Files Changed

### Primary Changes
- `go/plugins/compat_oai/generate.go`: Fix tool call ID source
- `go/plugins/compat_oai/tool_call_id_test.go`: Add comprehensive tests

### Test Coverage Added
- Tool calls with responses > 40 characters
- MCP tool integration scenarios
- Multi-turn conversation flows
- Edge cases and error handling

## Backwards Compatibility

This fix is **fully backwards compatible**. It corrects a bug that was completely blocking tool calling functionality, so any existing code that was working will continue to work, and previously broken tool calling will now function correctly.

## Verification

To verify the fix:

1. **Run reproduction tests**:
   ```bash
   go test ./reproduction/isolated-test.go -v
   ```

2. **Run new test suite**:
   ```bash
   go test ./go/plugins/compat_oai/tool_call_id_test.go -v
   ```

3. **Test with MCP integration**:
   ```bash
   go test ./reproduction/mcp-integration-test.go -v
   ```

## Business Impact

### Before Fix
- **Production-blocking**: All OpenAI tool calling unusable
- **Ecosystem fragmentation**: JavaScript works, Go doesn't
- **Developer frustration**: Cryptic errors with no clear solution

### After Fix  
- **Full OpenAI compatibility**: All tool calling scenarios work
- **Ecosystem alignment**: Go matches JavaScript behavior
- **Developer success**: Tool calling "just works" as expected

## Risk Assessment

**Risk Level**: Very Low

- **Minimal code change**: Single line fix with clear intent
- **Aligns with specification**: Matches OpenAI protocol requirements
- **Matches working implementation**: Uses same approach as JavaScript
- **Comprehensive testing**: Full test coverage for edge cases
- **No breaking changes**: Only fixes broken functionality

## Checklist

- [x] Bug reproduced and root cause identified
- [x] Fix implemented and tested
- [x] Test coverage added for regression prevention
- [x] Documentation updated
- [x] Backwards compatibility verified
- [x] Cross-platform alignment confirmed (matches JavaScript)

## Related Issues

This fix resolves tool calling failures reported in the community where users experienced:
- Mysterious tool_call_id length errors
- Tool calls working in JavaScript but failing in Go
- MCP integration completely broken with OpenAI

---

**Note**: This is a critical production bug that affects all Go users attempting to use OpenAI-compatible tool calling. The fix is minimal and safe, bringing the Go implementation in line with the working JavaScript version.