package repositories

import (
	"context"
	"database/sql"
	"station/internal/db/queries"
	"station/pkg/models"
)

type SettingsRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewSettingsRepo(db *sql.DB) *SettingsRepo {
	return &SettingsRepo{
		db:      db,
		queries: queries.New(db),
	}
}

func (r *SettingsRepo) GetByKey(key string) (*models.Setting, error) {
	setting, err := r.queries.GetSetting(context.Background(), key)
	if err != nil {
		return nil, err
	}

	var description *string
	if setting.Description.Valid {
		description = &setting.Description.String
	}

	return &models.Setting{
		ID:          setting.ID,
		Key:         setting.Key,
		Value:       setting.Value,
		Description: description,
		CreatedAt:   setting.CreatedAt.Time,
		UpdatedAt:   setting.UpdatedAt.Time,
	}, nil
}

func (r *SettingsRepo) GetAll() ([]*models.Setting, error) {
	settings, err := r.queries.GetAllSettings(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]*models.Setting, len(settings))
	for i, setting := range settings {
		var description *string
		if setting.Description.Valid {
			description = &setting.Description.String
		}

		result[i] = &models.Setting{
			ID:          setting.ID,
			Key:         setting.Key,
			Value:       setting.Value,
			Description: description,
			CreatedAt:   setting.CreatedAt.Time,
			UpdatedAt:   setting.UpdatedAt.Time,
		}
	}

	return result, nil
}

func (r *SettingsRepo) Set(key, value, description string) error {
	return r.queries.SetSetting(context.Background(), queries.SetSettingParams{
		Key:         key,
		Value:       value,
		Description: sql.NullString{String: description, Valid: description != ""},
	})
}

func (r *SettingsRepo) Delete(key string) error {
	return r.queries.DeleteSetting(context.Background(), key)
}
