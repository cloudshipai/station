package repositories

import (
	"context"
	"database/sql"
	"time"
	"station/internal/db/queries"
	"station/pkg/models"
)

type WebhookDeliveriesRepo struct {
	db      *sql.DB
	queries *queries.Queries
}

func NewWebhookDeliveriesRepo(db *sql.DB) *WebhookDeliveriesRepo {
	return &WebhookDeliveriesRepo{
		db:      db,
		queries: queries.New(db),
	}
}

func (r *WebhookDeliveriesRepo) Create(delivery *models.WebhookDelivery) (*models.WebhookDelivery, error) {
	result, err := r.queries.CreateWebhookDelivery(context.Background(), queries.CreateWebhookDeliveryParams{
		WebhookID:    delivery.WebhookID,
		EventType:    delivery.EventType,
		Payload:      delivery.Payload,
		Status:       delivery.Status,
		AttemptCount: sql.NullInt64{Int64: int64(delivery.AttemptCount), Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return r.convertToModel(result), nil
}

func (r *WebhookDeliveriesRepo) GetByID(id int64) (*models.WebhookDelivery, error) {
	delivery, err := r.queries.GetWebhookDelivery(context.Background(), id)
	if err != nil {
		return nil, err
	}

	return r.convertToModel(delivery), nil
}

func (r *WebhookDeliveriesRepo) List(limit int) ([]*models.WebhookDelivery, error) {
	deliveries, err := r.queries.ListWebhookDeliveries(context.Background(), int64(limit))
	if err != nil {
		return nil, err
	}

	result := make([]*models.WebhookDelivery, len(deliveries))
	for i, delivery := range deliveries {
		result[i] = r.convertToModel(delivery)
	}

	return result, nil
}

func (r *WebhookDeliveriesRepo) ListByWebhook(webhookID int64, limit int) ([]*models.WebhookDelivery, error) {
	deliveries, err := r.queries.ListWebhookDeliveriesByWebhook(context.Background(), queries.ListWebhookDeliveriesByWebhookParams{
		WebhookID: webhookID,
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}

	result := make([]*models.WebhookDelivery, len(deliveries))
	for i, delivery := range deliveries {
		result[i] = r.convertToModel(delivery)
	}

	return result, nil
}

func (r *WebhookDeliveriesRepo) ListPending() ([]*models.WebhookDelivery, error) {
	deliveries, err := r.queries.ListPendingDeliveries(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]*models.WebhookDelivery, len(deliveries))
	for i, delivery := range deliveries {
		result[i] = r.convertToModel(delivery)
	}

	return result, nil
}

func (r *WebhookDeliveriesRepo) ListFailedForRetry() ([]*models.WebhookDelivery, error) {
	deliveries, err := r.queries.ListFailedDeliveriesForRetry(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]*models.WebhookDelivery, len(deliveries))
	for i, delivery := range deliveries {
		result[i] = r.convertToModel(delivery)
	}

	return result, nil
}

func (r *WebhookDeliveriesRepo) UpdateStatus(id int64, status string, httpStatusCode *int, responseBody, responseHeaders, errorMessage *string) error {
	var statusCodeSQL sql.NullInt64
	if httpStatusCode != nil {
		statusCodeSQL = sql.NullInt64{Int64: int64(*httpStatusCode), Valid: true}
	}

	var responseBodySQL, responseHeadersSQL, errorMessageSQL sql.NullString
	if responseBody != nil {
		responseBodySQL = sql.NullString{String: *responseBody, Valid: true}
	}
	if responseHeaders != nil {
		responseHeadersSQL = sql.NullString{String: *responseHeaders, Valid: true}
	}
	if errorMessage != nil {
		errorMessageSQL = sql.NullString{String: *errorMessage, Valid: true}
	}

	return r.queries.UpdateDeliveryStatus(context.Background(), queries.UpdateDeliveryStatusParams{
		ID:              id,
		Status:          status,
		HttpStatusCode:  statusCodeSQL,
		ResponseBody:    responseBodySQL,
		ResponseHeaders: responseHeadersSQL,
		ErrorMessage:    errorMessageSQL,
		Column6:         status, // Used for the CASE WHEN clause
	})
}

func (r *WebhookDeliveriesRepo) UpdateForRetry(id int64, nextRetryAt time.Time, httpStatusCode int, responseBody, responseHeaders, errorMessage string) error {
	return r.queries.UpdateDeliveryForRetry(context.Background(), queries.UpdateDeliveryForRetryParams{
		ID:              id,
		NextRetryAt:     sql.NullTime{Time: nextRetryAt, Valid: true},
		HttpStatusCode:  sql.NullInt64{Int64: int64(httpStatusCode), Valid: httpStatusCode > 0},
		ResponseBody:    sql.NullString{String: responseBody, Valid: responseBody != ""},
		ResponseHeaders: sql.NullString{String: responseHeaders, Valid: responseHeaders != ""},
		ErrorMessage:    sql.NullString{String: errorMessage, Valid: errorMessage != ""},
	})
}

func (r *WebhookDeliveriesRepo) MarkSuccess(id int64, httpStatusCode int, responseBody, responseHeaders string) error {
	return r.queries.MarkDeliveryAsSuccess(context.Background(), queries.MarkDeliveryAsSuccessParams{
		ID:              id,
		HttpStatusCode:  sql.NullInt64{Int64: int64(httpStatusCode), Valid: true},
		ResponseBody:    sql.NullString{String: responseBody, Valid: responseBody != ""},
		ResponseHeaders: sql.NullString{String: responseHeaders, Valid: responseHeaders != ""},
	})
}

func (r *WebhookDeliveriesRepo) MarkFailed(id int64, errorMessage string) error {
	return r.queries.MarkDeliveryAsFailed(context.Background(), queries.MarkDeliveryAsFailedParams{
		ID:           id,
		ErrorMessage: sql.NullString{String: errorMessage, Valid: errorMessage != ""},
	})
}

func (r *WebhookDeliveriesRepo) DeleteOldDeliveries(olderThan time.Time) error {
	return r.queries.DeleteOldDeliveries(context.Background(), sql.NullTime{Time: olderThan, Valid: true})
}

func (r *WebhookDeliveriesRepo) convertToModel(delivery queries.WebhookDelivery) *models.WebhookDelivery {
	result := &models.WebhookDelivery{
		ID:           delivery.ID,
		WebhookID:    delivery.WebhookID,
		EventType:    delivery.EventType,
		Payload:      delivery.Payload,
		Status:       delivery.Status,
		AttemptCount: int(delivery.AttemptCount.Int64),
		CreatedAt:    delivery.CreatedAt.Time,
	}

	if delivery.HttpStatusCode.Valid {
		statusCode := int(delivery.HttpStatusCode.Int64)
		result.HTTPStatusCode = &statusCode
	}

	if delivery.ResponseBody.Valid {
		result.ResponseBody = &delivery.ResponseBody.String
	}

	if delivery.ResponseHeaders.Valid {
		result.ResponseHeaders = &delivery.ResponseHeaders.String
	}

	if delivery.ErrorMessage.Valid {
		result.ErrorMessage = &delivery.ErrorMessage.String
	}

	if delivery.LastAttemptAt.Valid {
		result.LastAttemptAt = &delivery.LastAttemptAt.Time
	}

	if delivery.NextRetryAt.Valid {
		result.NextRetryAt = &delivery.NextRetryAt.Time
	}

	if delivery.DeliveredAt.Valid {
		result.DeliveredAt = &delivery.DeliveredAt.Time
	}

	return result
}