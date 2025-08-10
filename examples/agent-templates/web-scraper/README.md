# Web Scraper Agent Template

A sophisticated web scraping agent with API integration capabilities, demonstrating advanced template features including sensitive variable handling and environment-specific configurations.

## 📋 Overview

This example demonstrates:
- Sensitive variable handling (API keys, tokens)
- Complex template logic with conditionals
- Multiple environment configurations
- External API integration patterns
- Advanced validation and error handling

## 🎯 Use Case

Web scraping agent that can:
- Scrape websites with respect for robots.txt
- Handle authentication via API keys
- Process various content types (HTML, JSON, XML)
- Rate limiting and retry logic
- Data transformation and storage

## 🔧 Variables

| Variable | Type | Required | Sensitive | Description |
|----------|------|----------|-----------|-------------|
| `SERVICE_NAME` | string | Yes | No | Name of the scraping service |
| `API_KEY` | string | Yes | Yes | Authentication API key |
| `BASE_URL` | string | Yes | No | Base URL for scraping targets |
| `RATE_LIMIT` | number | No | No | Requests per second (default: 1) |
| `TIMEOUT` | number | No | No | Request timeout in seconds (default: 30) |
| `USER_AGENT` | string | No | No | Custom User-Agent string |
| `ENABLE_PROXY` | boolean | No | No | Use proxy for requests (default: false) |
| `PROXY_URL` | string | No | Yes | Proxy server URL (required if ENABLE_PROXY=true) |
| `MAX_RETRIES` | number | No | No | Maximum retry attempts (default: 3) |

## 🚀 Installation Examples

### API Installation with Full Configuration

```bash
curl -X POST http://localhost:8080/api/v1/agents/templates/install \
  -H "Content-Type: application/json" \
  -d '{
    "bundle_path": "/templates/web-scraper",
    "environment": "production",
    "variables": {
      "SERVICE_NAME": "E-commerce Data Collector",
      "API_KEY": "sk_live_abcdef123456",
      "BASE_URL": "https://api.example.com/v1",
      "RATE_LIMIT": 2,
      "TIMEOUT": 45,
      "USER_AGENT": "MyCompany-Bot/1.0",
      "ENABLE_PROXY": true,
      "PROXY_URL": "http://proxy.company.com:8080",
      "MAX_RETRIES": 5
    }
  }'
```

### CLI Interactive Installation

```bash
stn agent bundle install ./bundle --interactive --env staging
```

The interactive mode will:
1. Prompt for required variables with masked input for sensitive data
2. Show defaults for optional variables
3. Validate URL formats and numeric ranges
4. Confirm proxy settings if enabled

## 🌍 Environment-Specific Configs

### Development
- Lower rate limits for testing
- Shorter timeouts
- Verbose logging enabled
- Test API endpoints

### Staging  
- Production-like settings
- Moderate rate limits
- Proxy configuration testing
- Staging API endpoints

### Production
- Optimized rate limits
- Full proxy support
- Production API endpoints
- Error monitoring enabled

## 📊 Advanced Features

### Conditional Logic
The agent template uses Go template conditionals:

```go
{{ if .ENABLE_PROXY }}
"proxy_config": {
  "enabled": true,
  "url": "{{ .PROXY_URL }}",
  "auth": "auto"
}
{{ else }}
"proxy_config": {
  "enabled": false
}
{{ end }}
```

### Dynamic User Agent
Customizable User-Agent with fallbacks:

```go
"user_agent": "{{ if .USER_AGENT }}{{ .USER_AGENT }}{{ else }}{{ .SERVICE_NAME }}-Agent/1.0{{ end }}"
```

### Rate Limiting Configuration
Environment-appropriate rate limiting:

```go
"rate_limit": {
  "requests_per_second": {{ .RATE_LIMIT }},
  "burst_size": {{ mul .RATE_LIMIT 2 }},
  "retry_after": {{ div 60 .RATE_LIMIT }}
}
```

## ⚡ Quick Deploy

```bash
# Development with minimal config
stn agent bundle install ./bundle \
  --vars SERVICE_NAME="Test Scraper" \
  --vars API_KEY="test_key_123" \
  --vars BASE_URL="https://httpbin.org" \
  --env development

# Production with full configuration  
stn agent bundle install ./bundle \
  --vars-file ./variables/production.json \
  --env production
```

## 🔍 Validation Features

- **URL Validation**: BASE_URL and PROXY_URL must be valid HTTP/HTTPS URLs
- **Rate Limit Bounds**: RATE_LIMIT must be between 0.1 and 10 requests/second
- **Conditional Requirements**: PROXY_URL required when ENABLE_PROXY=true
- **Timeout Ranges**: TIMEOUT must be between 5 and 300 seconds
- **Retry Logic**: MAX_RETRIES must be between 0 and 10

## 🚨 Security Notes

- API_KEY and PROXY_URL are marked as sensitive
- Interactive mode masks sensitive input
- Environment variables are encrypted at rest
- Proxy credentials handled securely
- Rate limiting prevents aggressive scraping