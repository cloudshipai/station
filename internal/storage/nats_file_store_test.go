package storage

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

func setupTestServer(t *testing.T) (*nats.Conn, nats.JetStreamContext, func()) {
	t.Helper()

	opts := natstest.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	srv := natstest.RunServer(&opts)

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		t.Fatalf("connect: %v", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		srv.Shutdown()
		t.Fatalf("jetstream: %v", err)
	}

	cleanup := func() {
		nc.Close()
		srv.Shutdown()
	}

	return nc, js, cleanup
}

func TestNewNATSFileStore(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewNATSFileStore(js, DefaultConfig())
	if err != nil {
		t.Fatalf("NewNATSFileStore: %v", err)
	}
	defer store.Close()

	if store.bucket == nil {
		t.Error("expected bucket to be initialized")
	}
}

func TestNATSFileStore_PutAndGet(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewNATSFileStore(js, DefaultConfig())
	if err != nil {
		t.Fatalf("NewNATSFileStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	content := []byte("hello, world!")
	key := "files/test-file.txt"

	info, err := store.Put(ctx, key, bytes.NewReader(content), PutOptions{
		ContentType: "text/plain",
		Metadata:    map[string]string{"source": "test"},
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if info.Key != key {
		t.Errorf("key = %q, want %q", info.Key, key)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", info.Size, len(content))
	}
	if info.ContentType != "text/plain" {
		t.Errorf("content_type = %q, want %q", info.ContentType, "text/plain")
	}
	if info.Checksum == "" {
		t.Error("expected checksum to be set")
	}

	reader, getInfo, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer reader.Close()

	if getInfo.Key != key {
		t.Errorf("get info key = %q, want %q", getInfo.Key, key)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatalf("read: %v", err)
	}

	if !bytes.Equal(buf.Bytes(), content) {
		t.Errorf("content = %q, want %q", buf.String(), string(content))
	}
}

func TestNATSFileStore_Delete(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewNATSFileStore(js, DefaultConfig())
	if err != nil {
		t.Fatalf("NewNATSFileStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	key := "files/to-delete.txt"

	_, err = store.Put(ctx, key, bytes.NewReader([]byte("delete me")), PutOptions{})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("file should exist after Put")
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	exists, err = store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists after delete: %v", err)
	}
	if exists {
		t.Error("file should not exist after Delete")
	}
}

func TestNATSFileStore_List(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	config := DefaultConfig()
	config.BucketName = "test-list-bucket"
	store, err := NewNATSFileStore(js, config)
	if err != nil {
		t.Fatalf("NewNATSFileStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	files := []string{
		"files/a.txt",
		"files/b.txt",
		"runs/r1/output/result.csv",
		"runs/r2/output/data.json",
	}

	for _, key := range files {
		_, err := store.Put(ctx, key, bytes.NewReader([]byte("content")), PutOptions{})
		if err != nil {
			t.Fatalf("Put %s: %v", key, err)
		}
	}

	allFiles, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(allFiles) != 4 {
		t.Errorf("list all = %d, want 4", len(allFiles))
	}

	filesOnly, err := store.List(ctx, "files/")
	if err != nil {
		t.Fatalf("List files/: %v", err)
	}
	if len(filesOnly) != 2 {
		t.Errorf("list files/ = %d, want 2", len(filesOnly))
	}

	runsOnly, err := store.List(ctx, "runs/")
	if err != nil {
		t.Fatalf("List runs/: %v", err)
	}
	if len(runsOnly) != 2 {
		t.Errorf("list runs/ = %d, want 2", len(runsOnly))
	}
}

func TestNATSFileStore_GetInfo(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewNATSFileStore(js, DefaultConfig())
	if err != nil {
		t.Fatalf("NewNATSFileStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	key := "files/info-test.txt"
	content := []byte("info test content")

	_, err = store.Put(ctx, key, bytes.NewReader(content), PutOptions{
		ContentType: "text/plain",
		Metadata:    map[string]string{"tag": "test"},
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	info, err := store.GetInfo(ctx, key)
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}

	if info.Key != key {
		t.Errorf("key = %q, want %q", info.Key, key)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", info.Size, len(content))
	}
	if info.ContentType != "text/plain" {
		t.Errorf("content_type = %q, want %q", info.ContentType, "text/plain")
	}
	if info.Metadata["tag"] != "test" {
		t.Errorf("metadata tag = %q, want %q", info.Metadata["tag"], "test")
	}
}

func TestNATSFileStore_NotFound(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewNATSFileStore(js, DefaultConfig())
	if err != nil {
		t.Fatalf("NewNATSFileStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	_, _, err = store.Get(ctx, "nonexistent")
	if !IsNotFound(err) {
		t.Errorf("Get nonexistent: expected ErrFileNotFound, got %v", err)
	}

	_, err = store.GetInfo(ctx, "nonexistent")
	if !IsNotFound(err) {
		t.Errorf("GetInfo nonexistent: expected ErrFileNotFound, got %v", err)
	}

	err = store.Delete(ctx, "nonexistent")
	if !IsNotFound(err) {
		t.Errorf("Delete nonexistent: expected ErrFileNotFound, got %v", err)
	}
}

func TestNATSFileStore_FileTooLarge(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	config := DefaultConfig()
	config.MaxFileSize = 100

	store, err := NewNATSFileStore(js, config)
	if err != nil {
		t.Fatalf("NewNATSFileStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	largeContent := bytes.Repeat([]byte("x"), 200)

	_, err = store.Put(ctx, "files/large.txt", bytes.NewReader(largeContent), PutOptions{})
	if !IsTooLarge(err) {
		t.Errorf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestNATSFileStore_TTL(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewNATSFileStore(js, DefaultConfig())
	if err != nil {
		t.Fatalf("NewNATSFileStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	info, err := store.Put(ctx, "files/ttl-test.txt", bytes.NewReader([]byte("ttl")), PutOptions{
		TTL: 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if info.ExpiresAt.IsZero() {
		t.Error("expected ExpiresAt to be set")
	}

	expectedExpiry := time.Now().Add(24 * time.Hour)
	if info.ExpiresAt.Sub(expectedExpiry) > time.Minute {
		t.Errorf("ExpiresAt = %v, expected around %v", info.ExpiresAt, expectedExpiry)
	}
}

func TestGenerateFileID(t *testing.T) {
	id1 := GenerateFileID()
	id2 := GenerateFileID()

	if !strings.HasPrefix(id1, "f_") {
		t.Errorf("id should start with f_, got %s", id1)
	}

	if id1 == id2 {
		t.Error("ids should be unique")
	}
}

func TestGenerateKeys(t *testing.T) {
	runKey := GenerateRunOutputKey("run123", "output.csv")
	if runKey != "runs/run123/output/output.csv" {
		t.Errorf("run key = %q", runKey)
	}

	sessionKey := GenerateSessionKey("sess456", "file.txt")
	if sessionKey != "sessions/sess456/file.txt" {
		t.Errorf("session key = %q", sessionKey)
	}

	userKey := GenerateUserFileKey("f_abc")
	if userKey != "files/f_abc" {
		t.Errorf("user key = %q", userKey)
	}
}

var _ natsserver.Server
