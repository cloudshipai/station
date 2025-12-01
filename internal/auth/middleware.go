package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"station/internal/auth/oauth"
	"station/internal/config"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// AuthMiddleware provides API key and OAuth authentication for MCP endpoints
type AuthMiddleware struct {
	repos         *repositories.Repositories
	localMode     bool
	cloudshipOAuth *oauth.CloudShipOAuth
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

// NewAuthMiddlewareWithOAuth creates a new authentication middleware with CloudShip OAuth support
func NewAuthMiddlewareWithOAuth(repos *repositories.Repositories, localMode bool, cfg *config.Config) *AuthMiddleware {
	am := &AuthMiddleware{
		repos:     repos,
		localMode: localMode,
	}
	
	// Initialize CloudShip OAuth if enabled
	if cfg != nil && cfg.CloudShip.OAuth.Enabled {
		am.cloudshipOAuth = oauth.NewCloudShipOAuth(&cfg.CloudShip.OAuth)
	}
	
	return am
}

// Authenticate validates API key or CloudShip OAuth token from Bearer token
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

		// Extract the token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "empty token",
			})
			c.Abort()
			return
		}

		// Try local API key first (sk-* prefix)
		if strings.HasPrefix(token, "sk-") {
			user, err := am.repos.Users.GetByAPIKey(token)
			if err == nil {
				// Valid local API key
				c.Set("user", user)
				c.Set("user_id", user.ID)
				c.Set("is_admin", user.IsAdmin)
				c.Set("auth_type", "api_key")
				c.Next()
				return
			}
		}

		// Try CloudShip OAuth if enabled
		if am.cloudshipOAuth != nil && am.cloudshipOAuth.IsEnabled() {
			tokenInfo, err := am.cloudshipOAuth.ValidateToken(token)
			if err == nil && tokenInfo.Active {
				// Valid CloudShip OAuth token
				// Create a virtual user context from OAuth claims
				c.Set("user_id", tokenInfo.UserID)
				c.Set("cloudship_user_id", tokenInfo.UserID)
				c.Set("cloudship_email", tokenInfo.Email)
				c.Set("cloudship_org_id", tokenInfo.OrgID)
				c.Set("cloudship_scope", tokenInfo.Scope)
				c.Set("is_admin", false) // OAuth users are not local admins
				c.Set("auth_type", "cloudship_oauth")
				c.Next()
				return
			}
		}

		// Neither API key nor OAuth worked
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid token",
		})
		c.Abort()
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
