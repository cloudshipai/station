package repositories

import (
	"context"
	"database/sql"
	"encoding/json"

	"station/internal/db/queries"
)

type ModelProviderRepository struct {
	queries *queries.Queries
}

func NewModelProviderRepository(db *sql.DB) *ModelProviderRepository {
	return &ModelProviderRepository{
		queries: queries.New(db),
	}
}

func (r *ModelProviderRepository) Create(ctx context.Context, name, displayName, baseURL, apiKey string, headers map[string]string, enabled, isDefault bool) (*queries.ModelProvider, error) {
	var headersJSON sql.NullString
	if headers != nil && len(headers) > 0 {
		b, err := json.Marshal(headers)
		if err != nil {
			return nil, err
		}
		headersJSON = sql.NullString{String: string(b), Valid: true}
	}

	var apiKeyNull sql.NullString
	if apiKey != "" {
		apiKeyNull = sql.NullString{String: apiKey, Valid: true}
	}

	provider, err := r.queries.CreateModelProvider(ctx, queries.CreateModelProviderParams{
		Name:        name,
		DisplayName: displayName,
		BaseUrl:     baseURL,
		ApiKey:      apiKeyNull,
		Headers:     headersJSON,
		Enabled:     sql.NullBool{Bool: enabled, Valid: true},
		IsDefault:   sql.NullBool{Bool: isDefault, Valid: true},
	})

	return &provider, err
}

func (r *ModelProviderRepository) GetByID(ctx context.Context, id int64) (*queries.ModelProvider, error) {
	provider, err := r.queries.GetModelProvider(ctx, id)
	return &provider, err
}

func (r *ModelProviderRepository) GetByName(ctx context.Context, name string) (*queries.ModelProvider, error) {
	provider, err := r.queries.GetModelProviderByName(ctx, name)
	return &provider, err
}

func (r *ModelProviderRepository) List(ctx context.Context) ([]*queries.ModelProvider, error) {
	providers, err := r.queries.ListModelProviders(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*queries.ModelProvider, len(providers))
	for i := range providers {
		result[i] = &providers[i]
	}
	return result, nil
}

func (r *ModelProviderRepository) ListEnabled(ctx context.Context) ([]*queries.ModelProvider, error) {
	providers, err := r.queries.ListEnabledModelProviders(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*queries.ModelProvider, len(providers))
	for i := range providers {
		result[i] = &providers[i]
	}
	return result, nil
}

func (r *ModelProviderRepository) GetDefault(ctx context.Context) (*queries.ModelProvider, error) {
	provider, err := r.queries.GetDefaultModelProvider(ctx)
	return &provider, err
}

func (r *ModelProviderRepository) Update(ctx context.Context, id int64, displayName, baseURL, apiKey string, headers map[string]string, enabled bool) error {
	var headersJSON sql.NullString
	if headers != nil && len(headers) > 0 {
		b, err := json.Marshal(headers)
		if err != nil {
			return err
		}
		headersJSON = sql.NullString{String: string(b), Valid: true}
	}

	var apiKeyNull sql.NullString
	if apiKey != "" {
		apiKeyNull = sql.NullString{String: apiKey, Valid: true}
	}

	return r.queries.UpdateModelProvider(ctx, queries.UpdateModelProviderParams{
		ID:          id,
		DisplayName: displayName,
		BaseUrl:     baseURL,
		ApiKey:      apiKeyNull,
		Headers:     headersJSON,
		Enabled:     sql.NullBool{Bool: enabled, Valid: true},
	})
}

func (r *ModelProviderRepository) SetDefault(ctx context.Context, id int64) error {
	return r.queries.SetDefaultModelProvider(ctx, id)
}

func (r *ModelProviderRepository) Delete(ctx context.Context, id int64) error {
	return r.queries.DeleteModelProvider(ctx, id)
}