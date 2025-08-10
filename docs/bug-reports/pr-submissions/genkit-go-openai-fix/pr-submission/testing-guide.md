# Testing Guide for Maintainers

This guide helps Firebase Genkit maintainers test and verify the tool_call_id bug fix.

## Quick Start

### 1. Reproduce the Bug (Before Fix)

```bash
# Clone the reproduction tests
git clone [station-repo] /tmp/genkit-test
cd /tmp/genkit-test/pr-submissions/genkit-go-openai-fix

# Set up environment
export OPENAI_API_KEY="your-key-here"

# Run reproduction test (should fail)
go test ./reproduction/isolated-test.go -v -run TestToolCallIDBug
```

**Expected Result**: Test fails with tool_call_id errors demonstrating the bug.

### 2. Apply the Fix

```bash
# Apply minimal fix to your genkit repo
cd /path/to/genkit/repo
git apply /tmp/genkit-test/pr-submissions/genkit-go-openai-fix/fixes/minimal-fix.patch
```

### 3. Verify the Fix

```bash
# Run the same test (should now pass)
go test ./reproduction/isolated-test.go -v -run TestToolCallIDBug
```

**Expected Result**: Test passes, demonstrating the fix works.

## Detailed Testing Scenarios

### Test 1: Simple Tool Call ID Bug

**Purpose**: Verify the core bug is fixed

```bash
go test ./reproduction/isolated-test.go -v -run TestToolCallIDBug
```

**What it tests**:
- Tool with response longer than 40 characters  
- Previously failed with "string too long" error
- Now should succeed with proper tool_call_id correlation

**Expected output**:
```
=== RUN   TestToolCallIDBug
✅ Tool call succeeded: Based on my search, I found comprehensive information about testing...
--- PASS: TestToolCallIDBug (3.45s)
```

### Test 2: MCP Integration

**Purpose**: Verify complex MCP tools work correctly

```bash
go test ./reproduction/mcp-integration-test.go -v -run TestMCPIntegrationToolCallIDBug
```

**What it tests**:
- Real MCP filesystem tools
- Complex JSON responses (typically 500+ characters)
- Multi-step tool calling scenarios

**Expected behavior**:
- **Before fix**: Fails with "string too long" (1000+ character tool_call_id)
- **After fix**: Succeeds with proper short tool_call_ids

### Test 3: Multi-Turn Conversations  

**Purpose**: Verify extended tool calling works

```bash
go test ./fixes/verification/ -v -run TestMultiTurnToolCalls
```

**What it tests**:
- Multiple consecutive tool calls
- Tool call ID correlation across turns
- Complex multi-step reasoning

**Expected output**:
```
=== RUN   TestMultiTurnToolCalls
✅ Multi-turn tool calls succeeded: I searched for Go programming and found...
--- PASS: TestMultiTurnToolCalls (8.12s)
```

### Test 4: Edge Cases

**Purpose**: Verify robustness of the fix

```bash
go test ./fixes/verification/ -v
```

**What it covers**:
- Empty Ref fields (fallback ID generation)
- Very long tool responses (1000+ characters)
- Special characters in tool responses
- Unicode content in tool outputs
- Malformed tool requests

## Manual Testing Scenarios

### Scenario 1: Real OpenAI Integration

Create a test file:

```go
package main

import (
	"context"
	"os"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
)

func TestRealOpenAIIntegration(t *testing.T) {
	ctx := context.Background()
	
	g, err := genkit.Init(ctx, genkit.WithPlugins(&openai.OpenAI{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	
	// Define a tool that returns a response longer than 40 characters
	tool := genkit.DefineTool(g, "analyze_text", "Analyze text content", 
		func(ctx *ai.ToolContext, input map[string]string) (string, error) {
			text := input["text"]
			return fmt.Sprintf("Analysis of '%s': This text contains %d characters and appears to be %s in nature with several key themes", 
				text, len(text), "informational"), nil
		})
	
	// This should work with the fix
	response, err := genkit.Generate(ctx, g,
		ai.WithModelName("openai/gpt-4o"),
		ai.WithPrompt("Use the analyze_text tool to analyze this: 'Hello world'"),
		ai.WithTools(tool),
		ai.WithMaxTurns(2),
	)
	
	if err != nil {
		t.Fatalf("Real OpenAI integration failed: %v", err)
	}
	
	t.Logf("Success: %s", response.Text())
}
```

Run: `go test real_test.go -v`

### Scenario 2: MCP Filesystem Integration

```bash
# Install MCP filesystem server
npm install -g @modelcontextprotocol/server-filesystem

# Test with real MCP server
export OPENAI_API_KEY="your-key"
go test ./reproduction/mcp-integration-test.go -v
```

## Verification Checklist

### Before Fix Behavior
- [ ] Simple tools fail with "tool_call_id not found" errors
- [ ] MCP tools fail with "string too long" errors (40+ characters)  
- [ ] Multi-turn conversations break after first tool call
- [ ] Error messages mention tool responses in tool_call_id field

### After Fix Behavior
- [ ] Simple tools succeed with proper correlation
- [ ] MCP tools succeed regardless of response length
- [ ] Multi-turn conversations flow naturally
- [ ] tool_call_id values are short references (e.g., "call_abc123")

### Performance Verification
- [ ] No performance degradation in tool calling
- [ ] No memory leaks in multi-turn scenarios
- [ ] Proper cleanup of tool correlation data

## Debug Output Analysis

### Understanding the Fix

**Before fix** - Debug output shows:
```
🔍 DEBUG: tool_call_id being sent: "Found comprehensive documentation about S3 policies with examples and usage patterns"
❌ OpenAI Error: tool_call_id not found in previous tool_calls
```

**After fix** - Debug output shows:
```
🔍 DEBUG: tool_call_id being sent: "call_abc123"  
✅ OpenAI Success: tool_call_id matches original request
```

### Log Analysis

Look for these patterns in logs:

**Problem indicators** (before fix):
- Long strings in tool_call_id fields
- "string too long" errors from OpenAI API
- "not found in tool_calls" correlation errors

**Success indicators** (after fix):
- Short "call_xxxx" format tool_call_ids
- Successful tool call completions
- Multi-turn conversations flowing properly

## Common Issues During Testing

### Issue 1: API Key Not Set
```
Error: OPENAI_API_KEY not set
```
**Solution**: `export OPENAI_API_KEY="your-key-here"`

### Issue 2: MCP Server Not Available
```
Error: Failed to create MCP client
```
**Solution**: `npm install -g @modelcontextprotocol/server-filesystem`

### Issue 3: Rate Limiting
```
Error: Rate limit exceeded
```
**Solution**: Add delays between tests or use different API keys

### Issue 4: Network Issues
```
Error: Connection timeout
```
**Solution**: Check internet connection and OpenAI API status

## Test Environment Setup

### Prerequisites
- Go 1.19+ installed
- OpenAI API key with sufficient credits
- Internet connection for API calls
- Node.js (for MCP server installation)

### Environment Variables
```bash
export OPENAI_API_KEY="sk-..."
export GENKIT_DEBUG=true  # Optional: verbose logging
```

### Dependencies
```bash
# Install MCP tools
npm install -g @modelcontextprotocol/server-filesystem

# Verify go modules
go mod tidy
```

## Maintainer Verification Steps

1. **Review the patch**: Understand the single line change
2. **Run reproduction tests**: Confirm bug exists without fix
3. **Apply the fix**: Use provided patch file
4. **Run verification tests**: Confirm fix resolves the issue
5. **Test edge cases**: Verify robustness of the solution
6. **Performance test**: Ensure no degradation
7. **Cross-platform test**: Verify on different OS if possible

## Expected Test Results

| Test Case | Before Fix | After Fix |
|-----------|------------|-----------|
| Simple tools | ❌ Fails | ✅ Passes |
| MCP tools | ❌ Fails | ✅ Passes |
| Multi-turn | ❌ Fails | ✅ Passes |
| Long responses | ❌ Fails | ✅ Passes |
| Edge cases | ❌ Fails | ✅ Passes |

---

**Need Help?**

If you encounter issues during testing:
1. Check the debug output for tool_call_id values
2. Verify OpenAI API key has sufficient credits
3. Ensure MCP servers are properly installed
4. Review the error messages for correlation issues

This fix should be straightforward to verify - the bug is 100% reproducible and the fix provides immediate resolution.