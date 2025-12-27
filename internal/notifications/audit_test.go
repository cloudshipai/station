package notifications

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	tmpFile, err := os.CreateTemp("", "test_audit_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	db, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := `
	CREATE TABLE notification_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		log_id TEXT NOT NULL UNIQUE,
		approval_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		webhook_url TEXT,
		request_payload TEXT,
		response_status INTEGER,
		response_body TEXT,
		error_message TEXT,
		attempt_number INTEGER,
		duration_ms INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX idx_notification_logs_approval ON notification_logs(approval_id);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

func TestAuditService_LogWebhookSuccess(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuditService(db)
	ctx := context.Background()

	err := svc.LogWebhookSuccess(ctx, "appr-123", "https://example.com/webhook", 200, `{"ok":true}`, 150, 1)
	if err != nil {
		t.Fatalf("LogWebhookSuccess failed: %v", err)
	}

	logs, err := svc.GetLogsByApproval(ctx, "appr-123")
	if err != nil {
		t.Fatalf("GetLogsByApproval failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	log := logs[0]
	if log.EventType != EventWebhookSuccess {
		t.Errorf("Expected event type %s, got %s", EventWebhookSuccess, log.EventType)
	}
	if log.ResponseStatus == nil || *log.ResponseStatus != 200 {
		t.Errorf("Expected status 200, got %v", log.ResponseStatus)
	}
	if log.DurationMs == nil || *log.DurationMs != 150 {
		t.Errorf("Expected duration 150ms, got %v", log.DurationMs)
	}
}

func TestAuditService_LogWebhookFailure(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuditService(db)
	ctx := context.Background()

	err := svc.LogWebhookFailure(ctx, "appr-456", "https://example.com/webhook", "connection refused", 0, 50, 3)
	if err != nil {
		t.Fatalf("LogWebhookFailure failed: %v", err)
	}

	logs, err := svc.GetLogsByApproval(ctx, "appr-456")
	if err != nil {
		t.Fatalf("GetLogsByApproval failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	log := logs[0]
	if log.EventType != EventWebhookFailed {
		t.Errorf("Expected event type %s, got %s", EventWebhookFailed, log.EventType)
	}
	if log.ErrorMessage == nil || *log.ErrorMessage != "connection refused" {
		t.Errorf("Expected error message 'connection refused', got %v", log.ErrorMessage)
	}
	if log.AttemptNumber != 3 {
		t.Errorf("Expected attempt 3, got %d", log.AttemptNumber)
	}
}

func TestAuditService_LogApprovalDecision(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuditService(db)
	ctx := context.Background()

	err := svc.LogApprovalDecision(ctx, "appr-789", "approved", "alice@example.com")
	if err != nil {
		t.Fatalf("LogApprovalDecision failed: %v", err)
	}

	logs, err := svc.GetLogsByApproval(ctx, "appr-789")
	if err != nil {
		t.Fatalf("GetLogsByApproval failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	log := logs[0]
	if log.EventType != EventApprovalDecided {
		t.Errorf("Expected event type %s, got %s", EventApprovalDecided, log.EventType)
	}
	if log.RequestPayload == nil || *log.RequestPayload != "approved by alice@example.com" {
		t.Errorf("Unexpected payload: %v", log.RequestPayload)
	}
}

func TestAuditService_GetRecentLogs(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuditService(db)
	ctx := context.Background()

	_ = svc.LogWebhookSuccess(ctx, "appr-1", "https://a.com", 200, "", 100, 1)
	_ = svc.LogWebhookSuccess(ctx, "appr-2", "https://b.com", 200, "", 100, 1)
	_ = svc.LogWebhookSuccess(ctx, "appr-3", "https://c.com", 200, "", 100, 1)

	logs, err := svc.GetRecentLogs(ctx, 2)
	if err != nil {
		t.Fatalf("GetRecentLogs failed: %v", err)
	}

	if len(logs) != 2 {
		t.Errorf("Expected 2 logs, got %d", len(logs))
	}
}
