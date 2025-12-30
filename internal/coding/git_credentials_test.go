package coding

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitCredentials(t *testing.T) {
	t.Run("DirectToken", func(t *testing.T) {
		gc := NewGitCredentials("ghp_test123", "")
		assert.Equal(t, "ghp_test123", gc.Token)
		assert.True(t, gc.HasToken())
	})

	t.Run("TokenFromEnvVar", func(t *testing.T) {
		os.Setenv("TEST_GH_TOKEN", "ghp_fromenv456")
		defer os.Unsetenv("TEST_GH_TOKEN")

		gc := NewGitCredentials("", "TEST_GH_TOKEN")
		assert.Equal(t, "ghp_fromenv456", gc.Token)
		assert.True(t, gc.HasToken())
	})

	t.Run("DirectTokenOverridesEnvVar", func(t *testing.T) {
		os.Setenv("TEST_GH_TOKEN", "ghp_fromenv")
		defer os.Unsetenv("TEST_GH_TOKEN")

		gc := NewGitCredentials("ghp_direct", "TEST_GH_TOKEN")
		assert.Equal(t, "ghp_direct", gc.Token)
	})

	t.Run("NoToken", func(t *testing.T) {
		gc := NewGitCredentials("", "")
		assert.False(t, gc.HasToken())
	})

	t.Run("MissingEnvVar", func(t *testing.T) {
		gc := NewGitCredentials("", "NONEXISTENT_VAR")
		assert.False(t, gc.HasToken())
	})

	t.Run("DefaultGitIdentity", func(t *testing.T) {
		gc := NewGitCredentials("token", "")
		assert.Equal(t, "Station Bot", gc.UserName)
		assert.Equal(t, "station@cloudship.ai", gc.UserEmail)
	})
}

func TestGitCredentials_InjectCredentials(t *testing.T) {
	gc := NewGitCredentials("ghp_testtoken123", "")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTPS GitHub URL",
			input:    "https://github.com/org/repo.git",
			expected: "https://x-access-token:ghp_testtoken123@github.com/org/repo.git",
		},
		{
			name:     "HTTPS GitHub URL without .git",
			input:    "https://github.com/org/repo",
			expected: "https://x-access-token:ghp_testtoken123@github.com/org/repo",
		},
		{
			name:     "HTTP URL (also supported)",
			input:    "http://github.com/org/repo.git",
			expected: "http://x-access-token:ghp_testtoken123@github.com/org/repo.git",
		},
		{
			name:     "SSH URL unchanged",
			input:    "git@github.com:org/repo.git",
			expected: "git@github.com:org/repo.git",
		},
		{
			name:     "SSH protocol URL unchanged",
			input:    "ssh://git@github.com/org/repo.git",
			expected: "ssh://git@github.com/org/repo.git",
		},
		{
			name:     "URL with existing credentials unchanged",
			input:    "https://user:pass@github.com/org/repo.git",
			expected: "https://user:pass@github.com/org/repo.git",
		},
		{
			name:     "GitLab URL",
			input:    "https://gitlab.com/org/repo.git",
			expected: "https://x-access-token:ghp_testtoken123@gitlab.com/org/repo.git",
		},
		{
			name:     "Self-hosted GitHub",
			input:    "https://github.mycompany.com/org/repo.git",
			expected: "https://x-access-token:ghp_testtoken123@github.mycompany.com/org/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gc.InjectCredentials(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("NoTokenReturnsOriginal", func(t *testing.T) {
		gcNoToken := NewGitCredentials("", "")
		result := gcNoToken.InjectCredentials("https://github.com/org/repo.git")
		assert.Equal(t, "https://github.com/org/repo.git", result)
	})

	t.Run("NilCredentialsReturnsOriginal", func(t *testing.T) {
		var gcNil *GitCredentials
		result := gcNil.InjectCredentials("https://github.com/org/repo.git")
		assert.Equal(t, "https://github.com/org/repo.git", result)
	})

	t.Run("InvalidURLReturnsOriginal", func(t *testing.T) {
		result := gc.InjectCredentials("not a valid url ://")
		assert.Equal(t, "not a valid url ://", result)
	})
}

func TestRedactString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // What the output should NOT contain
		check    func(t *testing.T, result string)
	}{
		{
			name:  "GitHub PAT ghp_",
			input: "Token is ghp_abcdefghijklmnopqrstuvwxyz123456",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "[REDACTED_GITHUB_TOKEN]")
				assert.NotContains(t, result, "ghp_")
			},
		},
		{
			name:  "GitHub OAuth gho_",
			input: "Using gho_abcdefghijklmnopqrstuvwxyz123456 for auth",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "[REDACTED_GITHUB_TOKEN]")
				assert.NotContains(t, result, "gho_")
			},
		},
		{
			name:  "GitHub fine-grained github_pat_",
			input: "github_pat_abcdefghijklmnopqrstuvwxyz123456789",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "[REDACTED_GITHUB_TOKEN]")
				assert.NotContains(t, result, "github_pat_")
			},
		},
		{
			name:  "URL with user:password",
			input: "https://myuser:supersecretpassword@github.com/org/repo",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "[REDACTED]")
				assert.NotContains(t, result, "supersecretpassword")
				assert.Contains(t, result, "github.com/org/repo")
			},
		},
		{
			name:  "Bearer token",
			input: "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "Bearer [REDACTED]")
				assert.NotContains(t, result, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
			},
		},
		{
			name:  "API key in config",
			input: "api_key=secret_key_abcdefghijklmnopqrstuvwxyz",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "[REDACTED]")
				assert.NotContains(t, result, "secret_key_")
			},
		},
		{
			name:  "No sensitive data unchanged",
			input: "Just a normal log message with no secrets",
			check: func(t *testing.T, result string) {
				assert.Equal(t, "Just a normal log message with no secrets", result)
			},
		},
		{
			name:  "Multiple tokens in one string",
			input: "First: ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa Second: gho_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			check: func(t *testing.T, result string) {
				assert.NotContains(t, result, "ghp_")
				assert.NotContains(t, result, "gho_")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactString(tt.input)
			tt.check(t, result)
		})
	}
}

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL with password",
			input:    "https://user:password123@github.com/org/repo.git",
			expected: "https://%5BREDACTED%5D:%5BREDACTED%5D@github.com/org/repo.git",
		},
		{
			name:     "URL with token only",
			input:    "https://ghp_token123@github.com/org/repo.git",
			expected: "https://%5BREDACTED%5D@github.com/org/repo.git",
		},
		{
			name:     "URL without credentials",
			input:    "https://github.com/org/repo.git",
			expected: "https://github.com/org/repo.git",
		},
		{
			name:     "SSH URL unchanged",
			input:    "git@github.com:org/repo.git",
			expected: "git@github.com:org/repo.git",
		},
		{
			name:     "Invalid URL falls back to string redaction",
			input:    "not://valid url with ghp_tokenabcdefghijklmnopqrstuvwxyz12",
			expected: "not://valid url with [REDACTED_GITHUB_TOKEN]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRedactError(t *testing.T) {
	t.Run("RedactsErrorMessage", func(t *testing.T) {
		original := errors.New("git clone https://ghp_secret123456789012345678901234@github.com/org/repo failed")
		redacted := RedactError(original)

		require.NotNil(t, redacted)
		assert.NotContains(t, redacted.Error(), "ghp_secret")
		assert.Contains(t, redacted.Error(), "[REDACTED")
	})

	t.Run("PreservesErrorType", func(t *testing.T) {
		original := errors.New("original error with ghp_tokenaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		redacted := RedactError(original)

		// Can unwrap to get original
		unwrapped := errors.Unwrap(redacted)
		assert.Equal(t, original, unwrapped)
	})

	t.Run("NilErrorReturnsNil", func(t *testing.T) {
		assert.Nil(t, RedactError(nil))
	})

	t.Run("ErrorWithoutSecretsUnchanged", func(t *testing.T) {
		original := errors.New("simple error message")
		redacted := RedactError(original)
		assert.Equal(t, "simple error message", redacted.Error())
	})
}

func TestHasToken(t *testing.T) {
	t.Run("NilCredentials", func(t *testing.T) {
		var gc *GitCredentials
		assert.False(t, gc.HasToken())
	})

	t.Run("EmptyToken", func(t *testing.T) {
		gc := &GitCredentials{Token: ""}
		assert.False(t, gc.HasToken())
	})

	t.Run("WithToken", func(t *testing.T) {
		gc := &GitCredentials{Token: "some-token"}
		assert.True(t, gc.HasToken())
	})
}
