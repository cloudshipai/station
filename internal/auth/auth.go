package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// UserContextKey is the key used to store user info in request context
type UserContextKey string

const (
	UserKey UserContextKey = "user"
)

// AuthService handles API key authentication
type AuthService struct {
	repos *repositories.Repositories
}

// NewAuthService creates a new authentication service
func NewAuthService(repos *repositories.Repositories) *AuthService {
	return &AuthService{
		repos: repos,
	}
}

// GenerateAPIKey generates a new random API key
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(bytes), nil
}

// AuthenticateAPIKey validates an API key and returns the associated user
func (a *AuthService) AuthenticateAPIKey(ctx context.Context, apiKey string) (*models.User, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Remove 'Bearer ' prefix if present
	if strings.HasPrefix(apiKey, "Bearer ") {
		apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	}

	// Look up user by API key
	user, err := a.repos.Users.GetByAPIKey(apiKey)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	return user, nil
}

// RequireAuth is a middleware that requires API key authentication
func (a *AuthService) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Authenticate the API key
		user, err := a.AuthenticateAPIKey(r.Context(), authHeader)
		if err != nil {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// Add user to request context
		ctx := context.WithValue(r.Context(), UserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromHTTPContext extracts the authenticated user from the HTTP request context
func GetUserFromHTTPContext(ctx context.Context) (*models.User, error) {
	user, ok := ctx.Value(UserKey).(*models.User)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}
	return user, nil
}

// RequireAdmin checks if the authenticated user is an admin
func RequireAdmin(ctx context.Context) error {
	user, err := GetUserFromHTTPContext(ctx)
	if err != nil {
		return err
	}

	if !user.IsAdmin {
		return fmt.Errorf("admin access required")
	}

	return nil
}
