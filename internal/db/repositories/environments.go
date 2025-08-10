package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"station/internal/db/queries"
	"station/pkg/models"
)

type EnvironmentRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewEnvironmentRepo(db *sql.DB) *EnvironmentRepo {
	return &EnvironmentRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// convertEnvironmentFromSQLc converts sqlc Environment to models.Environment
func convertEnvironmentFromSQLc(env queries.Environment) *models.Environment {
	result := &models.Environment{
		ID:        env.ID,
		Name:      env.Name,
		CreatedBy: env.CreatedBy,
	}
	
	if env.Description.Valid {
		result.Description = &env.Description.String
	}
	
	if env.CreatedAt.Valid {
		result.CreatedAt = env.CreatedAt.Time
	}
	
	if env.UpdatedAt.Valid {
		result.UpdatedAt = env.UpdatedAt.Time
	}
	
	return result
}

func (r *EnvironmentRepo) Create(name string, description *string, createdBy int64) (*models.Environment, error) {
	params := queries.CreateEnvironmentParams{
		Name:      name,
		CreatedBy: createdBy,
	}
	
	if description != nil {
		params.Description = sql.NullString{String: *description, Valid: true}
	}
	
	created, err := r.queries.CreateEnvironment(context.Background(), params)
	if err != nil {
		return nil, err
	}
	
	return convertEnvironmentFromSQLc(created), nil
}

func (r *EnvironmentRepo) GetByID(id int64) (*models.Environment, error) {
	env, err := r.queries.GetEnvironment(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return convertEnvironmentFromSQLc(env), nil
}

func (r *EnvironmentRepo) GetByName(name string) (*models.Environment, error) {
	env, err := r.queries.GetEnvironmentByName(context.Background(), name)
	if err != nil {
		return nil, err
	}
	return convertEnvironmentFromSQLc(env), nil
}

func (r *EnvironmentRepo) List() ([]*models.Environment, error) {
	environments, err := r.queries.ListEnvironments(context.Background())
	if err != nil {
		return nil, err
	}
	
	var result []*models.Environment
	for _, env := range environments {
		result = append(result, convertEnvironmentFromSQLc(env))
	}
	
	return result, nil
}

func (r *EnvironmentRepo) Update(id int64, name string, description *string) error {
	params := queries.UpdateEnvironmentParams{
		Name: name,
		ID:   id,
	}
	
	if description != nil {
		params.Description = sql.NullString{String: *description, Valid: true}
	}
	
	return r.queries.UpdateEnvironment(context.Background(), params)
}

func (r *EnvironmentRepo) Delete(id int64) error {
	// Check if this is the default environment
	env, err := r.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}
	
	// Prevent deletion of the default environment
	if env.Name == "default" {
		return fmt.Errorf("cannot delete the default environment")
	}
	
	return r.queries.DeleteEnvironment(context.Background(), id)
}