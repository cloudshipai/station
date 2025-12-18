-- +goose Up
ALTER TABLE mcp_servers ADD COLUMN url TEXT DEFAULT '';

-- +goose Down
ALTER TABLE mcp_servers DROP COLUMN url;
