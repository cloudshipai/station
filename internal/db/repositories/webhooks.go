package repositories

import (
	"context"
	"database/sql"
	"station/internal/db/queries"
	"station/pkg/models"
)

type WebhooksRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewWebhooksRepo(db *sql.DB) *WebhooksRepo {
	return &WebhooksRepo{
		db:      db,
		queries: queries.New(db),
	}
}

func (r *WebhooksRepo) Create(webhook *models.Webhook) (*models.Webhook, error) {
	result, err := r.queries.CreateWebhook(context.Background(), queries.CreateWebhookParams{
		Name:           webhook.Name,
		Url:            webhook.URL,
		Secret:         sql.NullString{String: webhook.Secret, Valid: webhook.Secret != ""},
		Enabled:        sql.NullBool{Bool: webhook.Enabled, Valid: true},
		Events:         webhook.Events,
		Headers:        sql.NullString{String: webhook.Headers, Valid: webhook.Headers != ""},
		TimeoutSeconds: sql.NullInt64{Int64: int64(webhook.TimeoutSeconds), Valid: true},
		RetryAttempts:  sql.NullInt64{Int64: int64(webhook.RetryAttempts), Valid: true},
		CreatedBy:      webhook.CreatedBy,
	})
	if err != nil {
		return nil, err
	}

	return r.convertToModel(result), nil
}

func (r *WebhooksRepo) GetByID(id int64) (*models.Webhook, error) {
	webhook, err := r.queries.GetWebhook(context.Background(), id)
	if err != nil {
		return nil, err
	}

	return r.convertToModel(webhook), nil
}

func (r *WebhooksRepo) GetByName(name string) (*models.Webhook, error) {
	webhook, err := r.queries.GetWebhookByName(context.Background(), name)
	if err != nil {
		return nil, err
	}

	return r.convertToModel(webhook), nil
}

func (r *WebhooksRepo) List() ([]*models.Webhook, error) {
	webhooks, err := r.queries.ListWebhooks(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]*models.Webhook, len(webhooks))
	for i, webhook := range webhooks {
		result[i] = r.convertToModel(webhook)
	}

	return result, nil
}

func (r *WebhooksRepo) ListEnabled() ([]*models.Webhook, error) {
	webhooks, err := r.queries.ListEnabledWebhooks(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]*models.Webhook, len(webhooks))
	for i, webhook := range webhooks {
		result[i] = r.convertToModel(webhook)
	}

	return result, nil
}

func (r *WebhooksRepo) Update(id int64, webhook *models.Webhook) error {
	return r.queries.UpdateWebhook(context.Background(), queries.UpdateWebhookParams{
		ID:             id,
		Name:           webhook.Name,
		Url:            webhook.URL,
		Secret:         sql.NullString{String: webhook.Secret, Valid: webhook.Secret != ""},
		Enabled:        sql.NullBool{Bool: webhook.Enabled, Valid: true},
		Events:         webhook.Events,
		Headers:        sql.NullString{String: webhook.Headers, Valid: webhook.Headers != ""},
		TimeoutSeconds: sql.NullInt64{Int64: int64(webhook.TimeoutSeconds), Valid: true},
		RetryAttempts:  sql.NullInt64{Int64: int64(webhook.RetryAttempts), Valid: true},
	})
}

func (r *WebhooksRepo) Delete(id int64) error {
	return r.queries.DeleteWebhook(context.Background(), id)
}

func (r *WebhooksRepo) Enable(id int64) error {
	return r.queries.EnableWebhook(context.Background(), id)
}

func (r *WebhooksRepo) Disable(id int64) error {
	return r.queries.DisableWebhook(context.Background(), id)
}

func (r *WebhooksRepo) convertToModel(webhook queries.Webhook) *models.Webhook {
	return &models.Webhook{
		ID:             webhook.ID,
		Name:           webhook.Name,
		URL:            webhook.Url,
		Secret:         webhook.Secret.String,
		Enabled:        webhook.Enabled.Bool,
		Events:         webhook.Events,
		Headers:        webhook.Headers.String,
		TimeoutSeconds: int(webhook.TimeoutSeconds.Int64),
		RetryAttempts:  int(webhook.RetryAttempts.Int64),
		CreatedBy:      webhook.CreatedBy,
		CreatedAt:      webhook.CreatedAt.Time,
		UpdatedAt:      webhook.UpdatedAt.Time,
	}
}