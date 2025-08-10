package webhooks

import (
	"station/internal/theme"
)

// WebhookHandler handles webhook-related CLI commands
type WebhookHandler struct {
	themeManager *theme.ThemeManager
}

func NewWebhookHandler(themeManager *theme.ThemeManager) *WebhookHandler {
	return &WebhookHandler{themeManager: themeManager}
}