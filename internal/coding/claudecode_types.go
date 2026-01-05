package coding

import "encoding/json"

type claudeEvent struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type claudeMessage struct {
	ID      string               `json:"id"`
	Role    string               `json:"role"`
	Model   string               `json:"model,omitempty"`
	Content []claudeContentBlock `json:"content"`
	Usage   *claudeUsage         `json:"usage,omitempty"`
}

type claudeContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type claudeToolResult struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type claudeResult struct {
	Type          string       `json:"type"`
	Subtype       string       `json:"subtype,omitempty"`
	DurationMS    int64        `json:"duration_ms,omitempty"`
	DurationAPIMS int64        `json:"duration_api_ms,omitempty"`
	IsError       bool         `json:"is_error,omitempty"`
	NumTurns      int          `json:"num_turns,omitempty"`
	Result        string       `json:"result,omitempty"`
	SessionID     string       `json:"session_id,omitempty"`
	TotalCostUSD  float64      `json:"total_cost_usd,omitempty"`
	Usage         *claudeUsage `json:"usage,omitempty"`
}

type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

type claudeSystemMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
	Message string `json:"message,omitempty"`
	Level   string `json:"level,omitempty"`
}
