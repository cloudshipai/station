# Multi-Model Support in Station

Station supports multiple AI providers through OpenAI-compatible endpoints, allowing you to use various models from different providers while keeping your agent infrastructure consistent.

## Supported Providers

### 1. OpenAI (Default)
```bash
stn up --provider openai --api-key sk-your-key --model gpt-4o
```

**Models available:**
- `gpt-4o` - Latest GPT-4 Optimized
- `gpt-4o-mini` - Faster, cost-effective GPT-4
- `gpt-4-turbo` - Previous generation GPT-4
- `gpt-3.5-turbo` - Fast and affordable

**Configuration:**
```yaml
# ~/.config/station/config.yaml
ai_provider: openai
ai_api_key: sk-your-openai-key
ai_model: gpt-4o-mini
```

---

### 2. Meta Llama (Hosted)
```bash
stn up --provider openai \
  --api-key LLM-your-key \
  --base-url https://api.llama.com/compat/v1 \
  --model Llama-4-Maverick-17B-128E-Instruct-FP8
```

**Important:** Use the `/compat/v1` endpoint for OpenAI compatibility. The standard `/v1` endpoint uses a different response format.

**Models available:**
- `Llama-4-Maverick-17B-128E-Instruct-FP8` - Latest Llama 4 model
- `Llama-3.3-70B-Instruct` - Llama 3.3 large model
- `Llama-3.1-8B-Instruct` - Smaller Llama 3.1 model

**Configuration:**
```yaml
# ~/.config/station/config.yaml
ai_provider: openai
ai_api_key: LLM|your-llama-key
ai_base_url: https://api.llama.com/compat/v1
ai_model: Llama-4-Maverick-17B-128E-Instruct-FP8
```

**Test execution:**
```bash
# Verified working - Run ID 10, Duration: 1.4s
stn agent run test-agent "Test with Llama-4"
```

---

### 3. Ollama (Local Models)
```bash
# Start Ollama
docker run -d -p 11434:11434 ollama/ollama
ollama pull llama3.2

# Configure Station
stn up --provider openai \
  --base-url http://localhost:11434/v1 \
  --model llama3.2:3b-instruct-fp16
```

**Models available:**
- `llama3.2:3b-instruct-fp16` - Llama 3.2 3B (full precision)
- `llama3:latest` - Llama 3 8B
- `deepseek-r1:7b` - DeepSeek reasoning model
- `phi3:latest` - Microsoft Phi-3 3.8B

**Configuration:**
```yaml
# ~/.config/station/config.yaml
ai_provider: openai
ai_api_key: ollama  # Any value works for Ollama
ai_base_url: http://localhost:11434/v1
ai_model: llama3.2:3b-instruct-fp16
```

**Test execution:**
```bash
# Verified working - Run ID 6, Duration: 32.1s
stn agent run test-agent "Test with Ollama"
```

---

### 4. Anthropic Claude
```bash
stn up --provider anthropic --api-key sk-ant-your-key --model claude-3-5-sonnet-20241022
```

**Models available:**
- `claude-3-5-sonnet-20241022` - Latest Claude 3.5 Sonnet
- `claude-3-opus-20240229` - Most capable Claude model
- `claude-3-haiku-20240307` - Fastest Claude model

**Configuration:**
```yaml
# ~/.config/station/config.yaml
ai_provider: anthropic
ai_api_key: sk-ant-your-key
ai_model: claude-3-5-sonnet-20241022
```

---

### 5. Google Gemini
```bash
stn up --provider gemini --api-key your-key --model gemini-2.0-flash-exp
```

**Models available:**
- `gemini-2.0-flash-exp` - Latest Gemini 2.0 Flash
- `gemini-pro` - Gemini Pro
- `gemini-pro-vision` - Multimodal Gemini

**Configuration:**
```yaml
# ~/.config/station/config.yaml
ai_provider: gemini
ai_api_key: your-gemini-key
ai_model: gemini-2.0-flash-exp
```

---

## How Custom Base URLs Work

Station uses Firebase GenKit's OpenAI plugin with custom base URL support:

**Code Implementation** (`internal/services/genkit_provider.go:152-155`):
```go
if cfg.AIBaseURL != "" {
    logging.Debug("Using custom OpenAI base URL: %s", cfg.AIBaseURL)
    opts = append(opts, option.WithBaseURL(cfg.AIBaseURL))
}
```

**Provider Auto-Detection** (`genkit_provider.go:231-258`):
```go
func detectProviderFromModel(modelName, configuredProvider string) string {
    modelLower := strings.ToLower(modelName)

    // If model starts with gemini-, it's definitely a Gemini model
    if strings.HasPrefix(modelLower, "gemini") {
        return "gemini"
    }

    // For all other models (gpt-*, claude-*, llama*, etc.), use OpenAI-compatible
    return "openai"
}
```

**Requirements for OpenAI-Compatible Endpoints:**
1. Must support `/v1/chat/completions` endpoint
2. Must return `choices` array with `message.content`
3. Must return `usage` object with token counts
4. Standard request format: `{"model": "...", "messages": [...]}`

---

## Testing Your Configuration

### 1. Test API Endpoint Directly
```bash
# For Meta Llama
curl "https://api.llama.com/compat/v1/chat/completions" \
  -H "Authorization: Bearer $LLAMA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Llama-4-Maverick-17B-128E-Instruct-FP8",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# For Ollama
curl "http://localhost:11434/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3.2:3b-instruct-fp16",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

### 2. Test with Station Agent
```bash
# Create or use existing test agent
stn agent run test-agent "Test with new model - respond with a brief hello"

# Check execution results
stn runs list
stn runs inspect <run-id> -v
```

### 3. Verify OTEL Traces (Optional)
```bash
# Start Jaeger
make jaeger

# Configure OTEL
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Run agent and view traces
stn agent run test-agent "Test task"
open http://localhost:16686
```

---

## Troubleshooting

### Error: "index out of range [0] with length 0"
**Cause:** API endpoint is not returning OpenAI-compatible format

**Solution:**
- Verify you're using the correct endpoint (e.g., `/compat/v1` for Meta Llama, not `/v1`)
- Test the API directly with curl to verify response format
- Check that response includes `choices` array with `message` object

### Error: "Authentication Error" (401)
**Cause:** Invalid or missing API key

**Solution:**
```bash
# Verify your API key is set
echo $LLAMA_API_KEY  # or appropriate env var

# Test authentication directly
curl -H "Authorization: Bearer $LLAMA_API_KEY" https://api.llama.com/compat/v1/models
```

### Error: "panic: action is already registered"
**Cause:** GenKit limitation - prompts can't be re-registered in long-running servers

**Solution:**
- Use CLI execution (`stn agent run`) which creates fresh GenKit instances
- Restart Station server if needed: `stn down && stn up`

### Slow Execution Times
**Model Performance** (verified test results):
- Meta Llama-4-Maverick: ~1.4 seconds (fast, hosted API)
- Ollama Llama3.2: ~32 seconds (slower, local inference)
- OpenAI GPT-4o-mini: ~2-5 seconds (fast, hosted API)

**Solutions:**
- Use hosted APIs (OpenAI, Meta Llama) for production
- Use Ollama for development/testing only
- Consider upgrading to GPU-accelerated Ollama instance

---

## Configuration via Environment Variables

You can also configure Station using environment variables:

```bash
# Set provider configuration
export OPENAI_API_KEY="your-key"
export LLAMA_API_KEY="LLM|your-key"

# Override config file settings
export STN_AI_PROVIDER="openai"
export STN_AI_BASE_URL="https://api.llama.com/compat/v1"
export STN_AI_MODEL="Llama-4-Maverick-17B-128E-Instruct-FP8"

# Start Station
stn up
```

---

## Cost Considerations

**OpenAI (as of 2025):**
- GPT-4o: $5/1M input tokens, $15/1M output tokens
- GPT-4o-mini: $0.15/1M input tokens, $0.60/1M output tokens

**Meta Llama (hosted):**
- Check Meta's pricing at https://api.llama.com/pricing
- Generally competitive with OpenAI

**Ollama (local):**
- Free (uses your local compute)
- Requires GPU for reasonable performance
- Higher latency than hosted APIs

**Anthropic Claude:**
- Claude 3.5 Sonnet: $3/1M input tokens, $15/1M output tokens
- Claude 3 Haiku: $0.25/1M input tokens, $1.25/1M output tokens

---

## Adding New Providers

To add support for a new OpenAI-compatible provider:

1. **Verify API Compatibility:**
   ```bash
   curl "https://new-provider.com/v1/chat/completions" \
     -H "Authorization: Bearer $API_KEY" \
     -d '{"model": "model-name", "messages": [{"role": "user", "content": "test"}]}'
   ```

2. **Check Response Format:**
   - Must have `choices[0].message.content`
   - Must have `usage.{prompt_tokens,completion_tokens,total_tokens}`

3. **Configure Station:**
   ```yaml
   ai_provider: openai
   ai_api_key: your-key
   ai_base_url: https://new-provider.com/v1
   ai_model: model-name
   ```

4. **Test Execution:**
   ```bash
   stn agent run test-agent "Test new provider"
   ```

---

## Resources

- [Firebase GenKit Documentation](https://firebase.google.com/docs/genkit)
- [OpenAI API Reference](https://platform.openai.com/docs/api-reference)
- [Meta Llama API Docs](https://docs.llama.com/)
- [Ollama Documentation](https://github.com/ollama/ollama)
- [Station OTEL Setup](./OTEL_SETUP.md)

---

**Questions?** Open an issue at https://github.com/cloudshipai/station/issues
