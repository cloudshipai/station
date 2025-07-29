-- +goose Up
-- Model providers table
CREATE TABLE model_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE, -- e.g., "openai", "anthropic", "ollama"
    display_name TEXT NOT NULL, -- e.g., "OpenAI", "Anthropic", "Ollama"
    base_url TEXT NOT NULL,
    api_key TEXT, -- Can be NULL for providers like Ollama
    headers TEXT, -- JSON object of custom headers
    enabled BOOLEAN DEFAULT true,
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Models table
CREATE TABLE models (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id INTEGER NOT NULL,
    model_id TEXT NOT NULL, -- e.g., "gpt-4o", "claude-3-5-sonnet-20241022"
    name TEXT NOT NULL, -- Display name e.g., "GPT-4 Omni", "Claude 3.5 Sonnet"
    context_size INTEGER NOT NULL,
    max_tokens INTEGER NOT NULL,
    supports_tools BOOLEAN DEFAULT false,
    input_cost REAL DEFAULT 0.0, -- Cost per 1M tokens
    output_cost REAL DEFAULT 0.0, -- Cost per 1M tokens
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (provider_id) REFERENCES model_providers(id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);

-- Add model reference to agents table
ALTER TABLE agents ADD COLUMN model_id INTEGER REFERENCES models(id);

-- Create indexes for performance
CREATE INDEX idx_models_provider_id ON models(provider_id);
CREATE INDEX idx_models_enabled ON models(enabled);
CREATE INDEX idx_model_providers_enabled ON model_providers(enabled);
CREATE INDEX idx_model_providers_default ON model_providers(is_default);

-- +goose Down
DROP INDEX IF EXISTS idx_model_providers_default;
DROP INDEX IF EXISTS idx_model_providers_enabled;
DROP INDEX IF EXISTS idx_models_enabled;
DROP INDEX IF EXISTS idx_models_provider_id;

-- Remove model_id column from agents (SQLite doesn't support DROP COLUMN directly)
-- We'll need to recreate the table if we want to remove the column
-- For now, we'll leave it as it won't hurt

DROP TABLE IF EXISTS models;
DROP TABLE IF EXISTS model_providers;