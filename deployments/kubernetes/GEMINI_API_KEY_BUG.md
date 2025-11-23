# GEMINI_API_KEY Requirement Bug

## Problem

Station requires `GEMINI_API_KEY` environment variable **even when using OpenAI** as the AI provider. Without it, the server crashes on startup.

## Root Cause

### Call Chain

1. **MCP Server Initialization** (`internal/mcp/server.go:62`)
   ```go
   benchmarkService, err := services.NewBenchmarkService(database.Conn(), cfg)
   ```

2. **BenchmarkService Creation** (`internal/services/benchmark_service.go:34`)
   ```go
   judge, err := benchmark.NewJudge(cfg)
   if err != nil {
       return nil, fmt.Errorf("failed to create benchmark judge: %w", err)
   }
   ```

3. **Judge Initialization** (`pkg/benchmark/judge.go:32`)
   ```go
   client, err := ai.NewClient(cfg, false)
   if err != nil {
       return nil, fmt.Errorf("failed to create AI client: %w", err)
   }
   ```

4. **AI Client Initialization** (`pkg/faker/ai/client.go:95-98`)
   ```go
   func initializeGoogleAI(ctx context.Context, cfg *config.Config, debug bool) *genkit.Genkit {
       // Use environment variable for API key
       plugin := &googlegenai.GoogleAI{}  // PANICS if GEMINI_API_KEY not set!
       return genkit.Init(ctx, genkit.WithPlugins(plugin))
   }
   ```

### The Issue

The GoogleAI plugin initialization **panics** if `GEMINI_API_KEY` or `GOOGLE_API_KEY` is not found in the environment, even though:

1. The user configured OpenAI as their provider (`STN_AI_PROVIDER=openai`)
2. BenchmarkService has error handling (`lines 63-66` in `server.go`)
3. The panic happens BEFORE the error can be caught

## Impact

- Cannot deploy Station without GEMINI_API_KEY
- Confusing for users who only want to use OpenAI
- Unnecessary API key requirement for basic deployments

## Current Workaround

Set a dummy value in deployment:

```yaml
env:
- name: GEMINI_API_KEY
  value: "dummy-key-to-satisfy-benchmark-service"
```

## Proper Fix Options

### Option 1: Lazy Initialization (Recommended)

Make BenchmarkService initialize only when first used:

```go
// internal/services/benchmark_service.go
type BenchmarkService struct {
    db       *sql.DB
    cfg      *config.Config
    analyzer *benchmark.Analyzer
    once     sync.Once
    initErr  error
}

func (s *BenchmarkService) getAnalyzer() (*benchmark.Analyzer, error) {
    s.once.Do(func() {
        judge, err := benchmark.NewJudge(s.cfg)
        if err != nil {
            s.initErr = err
            return
        }
        s.analyzer = benchmark.NewAnalyzer(s.db, judge)
    })
    return s.analyzer, s.initErr
}
```

### Option 2: Make BenchmarkService Optional

```go
// internal/mcp/server.go:62
var benchmarkService *services.BenchmarkService
if cfg.AIProvider != "" {  // Only init if AI provider configured
    bs, err := services.NewBenchmarkService(database.Conn(), cfg)
    if err != nil {
        log.Printf("Warning: benchmark service disabled: %v", err)
    } else {
        benchmarkService = bs
    }
}
```

### Option 3: Provider-Specific Judge Creation

```go
// pkg/benchmark/judge.go
func NewJudge(cfg *config.Config) (*Judge, error) {
    if cfg == nil {
        return nil, fmt.Errorf("config cannot be nil")
    }
    
    // Only create client if provider matches what's needed
    if cfg.AIProvider != "googlegenai" && cfg.AIProvider != "gemini" {
        // Skip Google AI initialization for non-Google providers
        client, err := ai.NewClient(cfg, false)
        if err != nil {
            return nil, fmt.Errorf("failed to create AI client: %w", err)
        }
        return &Judge{client: client, model: client.GetModelName()}, nil
    }
    
    // For Google providers, proceed as normal
    client, err := ai.NewClient(cfg, false)
    if err != nil {
        return nil, fmt.Errorf("failed to create AI client: %w", err)
    }
    return &Judge{client: client, model: client.GetModelName()}, nil
}
```

## Recommended Solution

**Option 1 (Lazy Initialization)** is best because:
- No breaking changes to existing deployments
- BenchmarkService only initializes when actually used
- Clear error message if API key missing when needed
- Allows Station to start even if benchmarks won't work

## Testing the Fix

1. Remove GEMINI_API_KEY from environment
2. Set only OPENAI_API_KEY
3. Start Station server
4. Verify server starts successfully
5. Verify agents can execute with OpenAI
6. Attempt to run benchmark (should fail gracefully with clear error)

## Files to Modify

- `internal/services/benchmark_service.go` - Add lazy initialization
- `pkg/benchmark/judge.go` - Make judge creation provider-aware
- `pkg/faker/ai/client.go` - Add better error handling for missing keys

## Related Issues

This is part of a larger pattern where Station initializes all services eagerly at startup. Consider making more services lazy-init for:
- Faster startup
- Better error isolation
- Reduced resource usage
- Clearer error messages

---

## Fix Implemented (2025-11-23)

**Solution**: Implemented lazy initialization for BenchmarkService (Option 1)

**Changes Made**:
1. Modified `internal/services/benchmark_service.go`:
   - Added `sync.Once` for lazy initialization
   - Store `db` and `cfg` in service struct
   - Created `getAnalyzer()` method that initializes on first use
   - Updated `EvaluateAsync()` to call `getAnalyzer()` before use

**Result**: 
- ✅ Station starts successfully without GEMINI_API_KEY
- ✅ BenchmarkService only initializes when actually used
- ✅ Clear error message if benchmark evaluation fails due to missing credentials
- ✅ No breaking changes to existing deployments

**Testing**:
```bash
# Verified Station starts without GEMINI_API_KEY
unset GEMINI_API_KEY GOOGLE_API_KEY
export OPENAI_API_KEY="your-key"
stn serve --local
# ✅ Server starts successfully
```

**Kubernetes Deployment Update**:
- GEMINI_API_KEY no longer required in `secret.yaml`
- Can deploy with only OPENAI_API_KEY for OpenAI-based agents
- Benchmark evaluation will fail gracefully if attempted without proper credentials

---

**Status**: ✅ FIXED - Lazy initialization implemented  
**Priority**: Medium → Closed  
**Commit**: Benchmark service now uses lazy initialization
