package v1

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"station/internal/storage"
)

type FilesHandler struct {
	store storage.FileStore
}

func NewFilesHandler(store storage.FileStore) *FilesHandler {
	return &FilesHandler{store: store}
}

type UploadResponse struct {
	FileKey     string `json:"file_key"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
}

type FileInfoResponse struct {
	Key         string            `json:"key"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type,omitempty"`
	Checksum    string            `json:"checksum,omitempty"`
	CreatedAt   string            `json:"created_at"`
	ExpiresAt   string            `json:"expires_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func (h *FilesHandler) Upload(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file store not initialized"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	fileID := storage.GenerateFileID()
	key := storage.GenerateUserFileKey(fileID)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = detectContentType(header.Filename)
	}

	opts := storage.PutOptions{
		ContentType: contentType,
		Description: header.Filename,
		Metadata: map[string]string{
			"original_filename": header.Filename,
		},
	}

	info, err := h.store.Put(c.Request.Context(), key, file, opts)
	if err != nil {
		if storage.IsTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}
		if storage.IsQuotaExceeded(err) {
			c.JSON(http.StatusInsufficientStorage, gin.H{"error": "storage quota exceeded"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, UploadResponse{
		FileKey:     info.Key,
		SizeBytes:   info.Size,
		ContentType: info.ContentType,
		Checksum:    info.Checksum,
	})
}

func (h *FilesHandler) Download(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file store not initialized"})
		return
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file key is required"})
		return
	}

	key = normalizeFileKey(key)

	reader, info, err := h.store.Get(c.Request.Context(), key)
	if err != nil {
		if storage.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer reader.Close()

	if info.ContentType != "" {
		c.Header("Content-Type", info.ContentType)
	}
	c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	if info.Checksum != "" {
		c.Header("X-Checksum", info.Checksum)
	}

	filename := filepath.Base(key)
	if info.Metadata != nil {
		if orig, ok := info.Metadata["original_filename"]; ok {
			filename = orig
		}
	}
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")

	c.Status(http.StatusOK)
	io.Copy(c.Writer, reader)
}

func (h *FilesHandler) GetInfo(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file store not initialized"})
		return
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file key is required"})
		return
	}

	key = normalizeFileKey(key)

	info, err := h.store.GetInfo(c.Request.Context(), key)
	if err != nil {
		if storage.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := FileInfoResponse{
		Key:         info.Key,
		Size:        info.Size,
		ContentType: info.ContentType,
		Checksum:    info.Checksum,
		CreatedAt:   info.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Metadata:    info.Metadata,
	}
	if !info.ExpiresAt.IsZero() {
		resp.ExpiresAt = info.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
	}

	c.JSON(http.StatusOK, resp)
}

func (h *FilesHandler) List(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file store not initialized"})
		return
	}

	prefix := c.Query("prefix")

	files, err := h.store.List(c.Request.Context(), prefix)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var resp []FileInfoResponse
	for _, info := range files {
		item := FileInfoResponse{
			Key:         info.Key,
			Size:        info.Size,
			ContentType: info.ContentType,
			Checksum:    info.Checksum,
			CreatedAt:   info.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Metadata:    info.Metadata,
		}
		if !info.ExpiresAt.IsZero() {
			item.ExpiresAt = info.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		}
		resp = append(resp, item)
	}

	c.JSON(http.StatusOK, gin.H{"files": resp, "count": len(resp)})
}

func (h *FilesHandler) Delete(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file store not initialized"})
		return
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file key is required"})
		return
	}

	key = normalizeFileKey(key)

	err := h.store.Delete(c.Request.Context(), key)
	if err != nil {
		if storage.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": key})
}

func normalizeFileKey(key string) string {
	key = strings.TrimPrefix(key, "/")
	if !strings.HasPrefix(key, "files/") && !strings.HasPrefix(key, "runs/") && !strings.HasPrefix(key, "sessions/") {
		key = "files/" + key
	}
	return key
}

func detectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain"
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}
