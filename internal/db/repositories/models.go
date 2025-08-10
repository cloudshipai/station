package repositories

import (
	"context"
	"database/sql"

	"station/internal/db/queries"
)

type ModelRepository struct {
	queries *queries.Queries
}

func NewModelRepository(db *sql.DB) *ModelRepository {
	return &ModelRepository{
		queries: queries.New(db),
	}
}

func (r *ModelRepository) Create(ctx context.Context, providerID int64, modelID, name string, contextSize, maxTokens int64, supportsTools bool, inputCost, outputCost float64, enabled bool) (*queries.Model, error) {
	model, err := r.queries.CreateModel(ctx, queries.CreateModelParams{
		ProviderID:    providerID,
		ModelID:       modelID,
		Name:          name,
		ContextSize:   contextSize,
		MaxTokens:     maxTokens,
		SupportsTools: sql.NullBool{Bool: supportsTools, Valid: true},
		InputCost:     sql.NullFloat64{Float64: inputCost, Valid: true},
		OutputCost:    sql.NullFloat64{Float64: outputCost, Valid: true},
		Enabled:       sql.NullBool{Bool: enabled, Valid: true},
	})

	return &model, err
}

func (r *ModelRepository) GetByID(ctx context.Context, id int64) (*queries.Model, error) {
	model, err := r.queries.GetModel(ctx, id)
	return &model, err
}

func (r *ModelRepository) GetByProviderAndModelID(ctx context.Context, providerID int64, modelID string) (*queries.Model, error) {
	model, err := r.queries.GetModelByProviderAndModelID(ctx, queries.GetModelByProviderAndModelIDParams{
		ProviderID: providerID,
		ModelID:    modelID,
	})
	return &model, err
}

func (r *ModelRepository) List(ctx context.Context) ([]*queries.ListModelsRow, error) {
	models, err := r.queries.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*queries.ListModelsRow, len(models))
	for i := range models {
		result[i] = &models[i]
	}
	return result, nil
}

func (r *ModelRepository) ListByProvider(ctx context.Context, providerID int64) ([]*queries.Model, error) {
	models, err := r.queries.ListModelsByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}

	result := make([]*queries.Model, len(models))
	for i := range models {
		result[i] = &models[i]
	}
	return result, nil
}

func (r *ModelRepository) ListEnabled(ctx context.Context) ([]*queries.ListEnabledModelsRow, error) {
	models, err := r.queries.ListEnabledModels(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*queries.ListEnabledModelsRow, len(models))
	for i := range models {
		result[i] = &models[i]
	}
	return result, nil
}

func (r *ModelRepository) ListToolSupporting(ctx context.Context) ([]*queries.ListToolSupportingModelsRow, error) {
	models, err := r.queries.ListToolSupportingModels(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*queries.ListToolSupportingModelsRow, len(models))
	for i := range models {
		result[i] = &models[i]
	}
	return result, nil
}

func (r *ModelRepository) Update(ctx context.Context, id int64, name string, contextSize, maxTokens int64, supportsTools bool, inputCost, outputCost float64, enabled bool) error {
	return r.queries.UpdateModel(ctx, queries.UpdateModelParams{
		ID:            id,
		Name:          name,
		ContextSize:   contextSize,
		MaxTokens:     maxTokens,
		SupportsTools: sql.NullBool{Bool: supportsTools, Valid: true},
		InputCost:     sql.NullFloat64{Float64: inputCost, Valid: true},
		OutputCost:    sql.NullFloat64{Float64: outputCost, Valid: true},
		Enabled:       sql.NullBool{Bool: enabled, Valid: true},
	})
}

func (r *ModelRepository) Delete(ctx context.Context, id int64) error {
	return r.queries.DeleteModel(ctx, id)
}

func (r *ModelRepository) DeleteByProvider(ctx context.Context, providerID int64) error {
	return r.queries.DeleteModelsByProvider(ctx, providerID)
}