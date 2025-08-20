package types

// Agent represents a Station agent
type Agent struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model"`
	MaxSteps    int    `json:"max_steps"`
	Enabled     bool   `json:"enabled"`
	Tools       []Tool `json:"tools"`
}

// Tool represents an agent tool
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}