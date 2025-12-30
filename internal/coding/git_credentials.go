package coding

import (
	"net/url"
	"os"
	"regexp"
	"strings"
)

// GitCredentials manages git authentication for clone/push operations.
// In stdio/CLI mode, host credentials are used by default (no injection needed).
// In container/serve mode, explicit token configuration is required.
type GitCredentials struct {
	// Token is the GitHub PAT or fine-grained token
	Token string

	// TokenEnvVar is the environment variable name to read token from
	// If set and Token is empty, token will be read from this env var
	TokenEnvVar string

	// UserName for git commits (default: "Station Bot")
	UserName string

	// UserEmail for git commits (default: "station@cloudship.ai")
	UserEmail string
}

// NewGitCredentials creates a GitCredentials instance.
// If tokenEnvVar is provided and token is empty, reads from environment.
func NewGitCredentials(token, tokenEnvVar string) *GitCredentials {
	gc := &GitCredentials{
		Token:       token,
		TokenEnvVar: tokenEnvVar,
		UserName:    "Station Bot",
		UserEmail:   "station@cloudship.ai",
	}

	// If no direct token but env var specified, read from env
	if gc.Token == "" && gc.TokenEnvVar != "" {
		gc.Token = os.Getenv(gc.TokenEnvVar)
	}

	return gc
}

// HasToken returns true if credentials have a valid token
func (g *GitCredentials) HasToken() bool {
	return g != nil && g.Token != ""
}

// InjectCredentials rewrites a git URL to include authentication.
// Supports both HTTPS and SSH URL formats.
//
// HTTPS: https://github.com/org/repo → https://x-access-token:TOKEN@github.com/org/repo
// SSH: git@github.com:org/repo → unchanged (SSH uses keys, not tokens)
//
// Returns original URL if:
// - No token is configured
// - URL is SSH format
// - URL already has credentials
// - URL parsing fails
func (g *GitCredentials) InjectCredentials(repoURL string) string {
	if !g.HasToken() {
		return repoURL
	}

	// Don't modify SSH URLs
	if strings.HasPrefix(repoURL, "git@") || strings.Contains(repoURL, "ssh://") {
		return repoURL
	}

	// Parse the URL
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return repoURL
	}

	// Only handle HTTPS
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return repoURL
	}

	// Don't override existing credentials
	if parsed.User != nil && parsed.User.String() != "" {
		return repoURL
	}

	// Inject token using x-access-token format (GitHub's recommended approach)
	parsed.User = url.UserPassword("x-access-token", g.Token)

	return parsed.String()
}

// Redaction patterns for various credential formats
var redactPatterns = []*regexp.Regexp{
	// GitHub tokens: ghp_xxx, gho_xxx, github_pat_xxx
	regexp.MustCompile(`(ghp_|gho_|github_pat_)[A-Za-z0-9_]{30,}`),

	// Generic tokens in URLs: https://user:token@host or https://token@host
	regexp.MustCompile(`://([^:@/]+):([^@/]+)@`),
	regexp.MustCompile(`://([^@/]{20,})@`),

	// Bearer tokens
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9\-._~+/]+=*`),

	// API keys in key=value format
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret[_-]?key|token|password|credential)\s*[:=]\s*['"]?[A-Za-z0-9\-._]{16,}['"]?`),
}

// RedactString removes sensitive credentials from a string.
// Use this for logging, error messages, and OTEL span attributes.
func RedactString(s string) string {
	result := s

	for _, pattern := range redactPatterns {
		switch {
		case strings.Contains(pattern.String(), "://"):
			// URL patterns - preserve structure but redact credentials
			if strings.Contains(pattern.String(), "):([^@/]+)@") {
				// user:password format
				result = pattern.ReplaceAllString(result, "://[REDACTED]:[REDACTED]@")
			} else {
				// token-only format
				result = pattern.ReplaceAllString(result, "://[REDACTED]@")
			}
		case strings.Contains(pattern.String(), "bearer"):
			// Bearer token - keep "Bearer " prefix
			result = pattern.ReplaceAllString(result, "${1}[REDACTED]")
		case strings.Contains(pattern.String(), "ghp_|gho_|github_pat_"):
			// GitHub tokens
			result = pattern.ReplaceAllString(result, "[REDACTED_GITHUB_TOKEN]")
		default:
			// Key=value patterns - keep key, redact value
			result = pattern.ReplaceAllStringFunc(result, func(match string) string {
				parts := regexp.MustCompile(`[:=]\s*`).Split(match, 2)
				if len(parts) == 2 {
					return parts[0] + "=[REDACTED]"
				}
				return "[REDACTED]"
			})
		}
	}

	return result
}

// RedactURL specifically handles URL redaction, preserving URL structure.
// More targeted than RedactString for URL-specific use cases.
func RedactURL(repoURL string) string {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		// Fall back to generic redaction
		return RedactString(repoURL)
	}

	// Redact user info if present
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword("[REDACTED]", "[REDACTED]")
		} else if parsed.User.Username() != "" {
			parsed.User = url.User("[REDACTED]")
		}
	}

	return parsed.String()
}

// RedactError wraps an error with redacted message.
// The original error is preserved for type checking but String() is redacted.
func RedactError(err error) error {
	if err == nil {
		return nil
	}
	return &redactedError{
		original: err,
		redacted: RedactString(err.Error()),
	}
}

type redactedError struct {
	original error
	redacted string
}

func (e *redactedError) Error() string {
	return e.redacted
}

func (e *redactedError) Unwrap() error {
	return e.original
}

// createGitAskpassScript creates a temporary script that provides git credentials.
// Returns the script path, a cleanup function, and any error.
// The cleanup function should be called (usually via defer) to remove the script.
func createGitAskpassScript(token string) (scriptPath string, cleanup func(), err error) {
	tmpFile, err := os.CreateTemp("", "git-askpass-*.sh")
	if err != nil {
		return "", nil, err
	}

	scriptContent := "#!/bin/sh\necho '" + token + "'\n"
	if _, err := tmpFile.WriteString(scriptContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", nil, err
	}
	tmpFile.Close()

	if err := os.Chmod(tmpFile.Name(), 0700); err != nil {
		os.Remove(tmpFile.Name())
		return "", nil, err
	}

	cleanup = func() {
		os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup, nil
}
