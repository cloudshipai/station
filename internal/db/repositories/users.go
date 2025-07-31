package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"station/internal/db/queries"
	"station/pkg/models"
)

type UserRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{
		db:      db,
		queries: queries.New(db),
	}
}

// convertUserFromSQLc converts sqlc User to models.User
func convertUserFromSQLc(user queries.User) *models.User {
	result := &models.User{
		ID:       user.ID,
		Username: user.Username,
		IsAdmin:  user.IsAdmin,
	}
	
	if user.ApiKey.Valid {
		result.APIKey = &user.ApiKey.String
	}
	
	if user.CreatedAt.Valid {
		result.CreatedAt = user.CreatedAt.Time
	}
	
	if user.UpdatedAt.Valid {
		result.UpdatedAt = user.UpdatedAt.Time
	}
	
	return result
}

func (r *UserRepo) Create(username string, isAdmin bool, apiKey *string) (*models.User, error) {
	params := queries.CreateUserParams{
		Username: username,
		IsAdmin:  isAdmin,
	}
	
	if apiKey != nil {
		params.ApiKey = sql.NullString{String: *apiKey, Valid: true}
	}
	
	created, err := r.queries.CreateUser(context.Background(), params)
	if err != nil {
		return nil, err
	}
	
	return convertUserFromSQLc(created), nil
}

func (r *UserRepo) GetByID(id int64) (*models.User, error) {
	user, err := r.queries.GetUser(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return convertUserFromSQLc(user), nil
}

func (r *UserRepo) GetByUsername(username string) (*models.User, error) {
	user, err := r.queries.GetUserByUsername(context.Background(), username)
	if err != nil {
		return nil, err
	}
	return convertUserFromSQLc(user), nil
}

func (r *UserRepo) GetByAPIKey(apiKey string) (*models.User, error) {
	user, err := r.queries.GetUserByAPIKey(context.Background(), sql.NullString{String: apiKey, Valid: true})
	if err != nil {
		return nil, err
	}
	return convertUserFromSQLc(user), nil
}

func (r *UserRepo) List() ([]*models.User, error) {
	users, err := r.queries.ListUsers(context.Background())
	if err != nil {
		return nil, err
	}
	
	var result []*models.User
	for _, user := range users {
		result = append(result, convertUserFromSQLc(user))
	}
	
	return result, nil
}

func (r *UserRepo) Update(id int64, username string, isAdmin bool) error {
	params := queries.UpdateUserParams{
		Username: username,
		IsAdmin:  isAdmin,
		ID:       id,
	}
	return r.queries.UpdateUser(context.Background(), params)
}

func (r *UserRepo) UpdateAPIKey(id int64, apiKey *string) error {
	params := queries.UpdateUserAPIKeyParams{
		ID: id,
	}
	
	if apiKey != nil {
		params.ApiKey = sql.NullString{String: *apiKey, Valid: true}
	}
	
	return r.queries.UpdateUserAPIKey(context.Background(), params)
}

func (r *UserRepo) Delete(id int64) error {
	// Check if this is a system user that should not be deleted
	user, err := r.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	
	// Prevent deletion of system users
	if user.Username == "console" || user.Username == "system" {
		return fmt.Errorf("cannot delete system user '%s'", user.Username)
	}
	
	return r.queries.DeleteUser(context.Background(), id)
}