package tools

import (
	"testing"
)

func TestIsSensitiveFile(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		sensitive bool
	}{
		// Sensitive files that should get 0600 permissions
		{".env file", ".env", true},
		{".env.local", ".env.local", true},
		{".env.production", ".env.production", true},
		{"private key", "server.key", true},
		{"PEM certificate", "cert.pem", true},
		{"P12 keystore", "keystore.p12", true},
		{"secret file", "api_secret.txt", true},
		{"credential file", "credentials.json", true},
		{"password file", "passwords.txt", true},
		{"token file", "auth.token", true},
		{"SSH key", "id_rsa", true},
		{"ED25519 key", "id_ed25519", true},
		{"ECDSA key", "id_ecdsa", true},
		{"nested secret", "config/secrets/db_password.yaml", true},

		// Non-sensitive files that should get standard 0644 permissions
		{"regular go file", "main.go", false},
		{"readme", "README.md", false},
		{"config json", "config.json", false},
		{"package json", "package.json", false},
		{"dockerfile", "Dockerfile", false},
		{"makefile", "Makefile", false},
		{"html file", "index.html", false},
		{"css file", "styles.css", false},
		{"public cert", "ca-cert.crt", true}, // CRT files are sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSensitiveFile(tt.path)
			if got != tt.sensitive {
				t.Errorf("isSensitiveFile(%q) = %v, want %v", tt.path, got, tt.sensitive)
			}
		})
	}
}
