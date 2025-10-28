package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// AuthMiddleware provides API key authentication for MCP endpoints
type AuthMiddleware struct {
	repos     *repositories.Repositories
	localMode bool
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(repos *repositories.Repositories) *AuthMiddleware {
	return &AuthMiddleware{
		repos:     repos,
		localMode: false,
	}
}

// NewAuthMiddlewareWithLocalMode creates a new authentication middleware with local mode setting
func NewAuthMiddlewareWithLocalMode(repos *repositories.Repositories, localMode bool) *AuthMiddleware {
	return &AuthMiddleware{
		repos:     repos,
		localMode: localMode,
	}
}

// Authenticate validates API key from Bearer token
func (am *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication in local mode
		if am.localMode {
			// Set default user context for local mode
			c.Set("user_id", int64(1))
			c.Set("is_admin", true)
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format, expected Bearer token",
			})
			c.Abort()
			return
		}

		// Extract the API key
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "empty API key",
			})
			c.Abort()
			return
		}

		// Validate the API key
		user, err := am.repos.Users.GetByAPIKey(apiKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			c.Abort()
			return
		}

		// Store user information in context for use by handlers
		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("is_admin", user.IsAdmin)

		c.Next()
	}
}

// RequireAdmin ensures the authenticated user is an admin
func (am *AuthMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("is_admin")
		if !exists || !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "admin privileges required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserFromContext extracts the authenticated user from the Gin context
func GetUserFromContext(c *gin.Context) (*models.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	userModel, ok := user.(*models.User)
	return userModel, ok
}

// GetUserIDFromContext extracts the user ID from the Gin context
func GetUserIDFromContext(c *gin.Context) (int64, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}

	id, ok := userID.(int64)
	return id, ok
}
