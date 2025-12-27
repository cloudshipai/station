package notifications

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"station/internal/db/queries"
)

type EventType string

const (
	EventWebhookSent     EventType = "webhook_sent"
	EventWebhookSuccess  EventType = "webhook_success"
	EventWebhookFailed   EventType = "webhook_failed"
	EventApprovalDecided EventType = "approval_decided"
)

type NotificationLog struct {
	ID             int64
	LogID          string
	ApprovalID     string
	EventType      EventType
	WebhookURL     *string
	RequestPayload *string
	ResponseStatus *int
	ResponseBody   *string
	ErrorMessage   *string
	AttemptNumber  int
	DurationMs     *int64
	CreatedAt      time.Time
}

type AuditService struct {
	queries *queries.Queries
}

func NewAuditService(db *sql.DB) *AuditService {
	return &AuditService{
		queries: queries.New(db),
	}
}

func (a *AuditService) LogWebhookAttempt(ctx context.Context, approvalID, webhookURL, payload string, attempt int) (string, error) {
	logID := uuid.New().String()

	_, err := a.queries.InsertNotificationLog(ctx, queries.InsertNotificationLogParams{
		LogID:          logID,
		ApprovalID:     approvalID,
		EventType:      string(EventWebhookSent),
		WebhookUrl:     sql.NullString{String: webhookURL, Valid: true},
		RequestPayload: sql.NullString{String: payload, Valid: true},
		AttemptNumber:  sql.NullInt64{Int64: int64(attempt), Valid: true},
	})

	return logID, err
}

func (a *AuditService) LogWebhookSuccess(ctx context.Context, approvalID, webhookURL string, statusCode int, responseBody string, durationMs int64, attempt int) error {
	logID := uuid.New().String()

	_, err := a.queries.InsertNotificationLog(ctx, queries.InsertNotificationLogParams{
		LogID:          logID,
		ApprovalID:     approvalID,
		EventType:      string(EventWebhookSuccess),
		WebhookUrl:     sql.NullString{String: webhookURL, Valid: true},
		ResponseStatus: sql.NullInt64{Int64: int64(statusCode), Valid: true},
		ResponseBody:   sql.NullString{String: responseBody, Valid: responseBody != ""},
		DurationMs:     sql.NullInt64{Int64: durationMs, Valid: true},
		AttemptNumber:  sql.NullInt64{Int64: int64(attempt), Valid: true},
	})

	return err
}

func (a *AuditService) LogWebhookFailure(ctx context.Context, approvalID, webhookURL, errorMsg string, statusCode int, durationMs int64, attempt int) error {
	logID := uuid.New().String()

	var respStatus sql.NullInt64
	if statusCode > 0 {
		respStatus = sql.NullInt64{Int64: int64(statusCode), Valid: true}
	}

	_, err := a.queries.InsertNotificationLog(ctx, queries.InsertNotificationLogParams{
		LogID:          logID,
		ApprovalID:     approvalID,
		EventType:      string(EventWebhookFailed),
		WebhookUrl:     sql.NullString{String: webhookURL, Valid: true},
		ResponseStatus: respStatus,
		ErrorMessage:   sql.NullString{String: errorMsg, Valid: errorMsg != ""},
		DurationMs:     sql.NullInt64{Int64: durationMs, Valid: true},
		AttemptNumber:  sql.NullInt64{Int64: int64(attempt), Valid: true},
	})

	return err
}

func (a *AuditService) LogApprovalDecision(ctx context.Context, approvalID, decision, decidedBy string) error {
	logID := uuid.New().String()

	payload := decision
	if decidedBy != "" {
		payload = decision + " by " + decidedBy
	}

	_, err := a.queries.InsertNotificationLog(ctx, queries.InsertNotificationLogParams{
		LogID:          logID,
		ApprovalID:     approvalID,
		EventType:      string(EventApprovalDecided),
		RequestPayload: sql.NullString{String: payload, Valid: true},
		AttemptNumber:  sql.NullInt64{Int64: 1, Valid: true},
	})

	return err
}

func (a *AuditService) GetLogsByApproval(ctx context.Context, approvalID string) ([]NotificationLog, error) {
	rows, err := a.queries.ListNotificationLogsByApproval(ctx, approvalID)
	if err != nil {
		return nil, err
	}

	logs := make([]NotificationLog, len(rows))
	for i, row := range rows {
		logs[i] = convertNotificationLog(row)
	}

	return logs, nil
}

func (a *AuditService) GetRecentLogs(ctx context.Context, limit int) ([]NotificationLog, error) {
	rows, err := a.queries.ListRecentNotificationLogs(ctx, int64(limit))
	if err != nil {
		return nil, err
	}

	logs := make([]NotificationLog, len(rows))
	for i, row := range rows {
		logs[i] = convertNotificationLog(row)
	}

	return logs, nil
}

func convertNotificationLog(row queries.NotificationLog) NotificationLog {
	log := NotificationLog{
		ID:         row.ID,
		LogID:      row.LogID,
		ApprovalID: row.ApprovalID,
		EventType:  EventType(row.EventType),
		CreatedAt:  row.CreatedAt.Time,
	}

	if row.AttemptNumber.Valid {
		log.AttemptNumber = int(row.AttemptNumber.Int64)
	}
	if row.WebhookUrl.Valid {
		log.WebhookURL = &row.WebhookUrl.String
	}
	if row.RequestPayload.Valid {
		log.RequestPayload = &row.RequestPayload.String
	}
	if row.ResponseStatus.Valid {
		status := int(row.ResponseStatus.Int64)
		log.ResponseStatus = &status
	}
	if row.ResponseBody.Valid {
		log.ResponseBody = &row.ResponseBody.String
	}
	if row.ErrorMessage.Valid {
		log.ErrorMessage = &row.ErrorMessage.String
	}
	if row.DurationMs.Valid {
		log.DurationMs = &row.DurationMs.Int64
	}

	return log
}
