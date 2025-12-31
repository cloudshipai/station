package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"

	"station/internal/storage"
	"station/internal/theme"
)

type FilesHandler struct {
	themeManager *theme.ThemeManager
}

func NewFilesHandler(themeManager *theme.ThemeManager) *FilesHandler {
	return &FilesHandler{themeManager: themeManager}
}

func (h *FilesHandler) Upload(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	stationURL, _ := cmd.Flags().GetString("station")
	customKey, _ := cmd.Flags().GetString("key")
	ttlStr, _ := cmd.Flags().GetString("ttl")

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("📤 Upload File"))

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	var ttl time.Duration
	if ttlStr != "" {
		var err error
		ttl, err = time.ParseDuration(ttlStr)
		if err != nil {
			return fmt.Errorf("invalid TTL format: %s (use format like 24h, 30m, 7d)", ttlStr)
		}
	}

	if stationURL != "" {
		return h.uploadViaAPI(filePath, stationURL, customKey, ttl, styles)
	}

	return h.uploadLocal(filePath, customKey, ttl, styles)
}

func (h *FilesHandler) uploadLocal(filePath, customKey string, ttl time.Duration, styles CLIStyles) error {
	ctx := context.Background()

	store, cleanup, err := h.createLocalFileStore()
	if err != nil {
		return err
	}
	defer cleanup()

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	contentType := detectContentType(filePath)
	key := customKey
	if key == "" {
		fileID := storage.GenerateFileID()
		key = storage.GenerateUserFileKey(fileID)
	}

	opts := storage.PutOptions{
		ContentType: contentType,
		TTL:         ttl,
		Description: filepath.Base(filePath),
	}

	info, err := store.Put(ctx, key, file, opts)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	fmt.Printf("%s Uploaded successfully\n", styles.Success.Render("✅"))
	fmt.Printf("   Key: %s\n", styles.Info.Render(info.Key))
	fmt.Printf("   Size: %s\n", formatBytes(fileInfo.Size()))
	fmt.Printf("   Type: %s\n", info.ContentType)
	if !info.ExpiresAt.IsZero() {
		fmt.Printf("   Expires: %s\n", info.ExpiresAt.Format(time.RFC3339))
	}

	return nil
}

func (h *FilesHandler) uploadViaAPI(filePath, stationURL, customKey string, ttl time.Duration, styles CLIStyles) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	if customKey != "" {
		writer.WriteField("key", customKey)
	}
	if ttl > 0 {
		writer.WriteField("ttl", ttl.String())
	}
	writer.Close()

	url := strings.TrimSuffix(stationURL, "/") + "/api/v1/files"
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		FileKey     string `json:"file_key"`
		SizeBytes   int64  `json:"size_bytes"`
		ContentType string `json:"content_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("%s Uploaded successfully\n", styles.Success.Render("✅"))
	fmt.Printf("   Key: %s\n", styles.Info.Render(result.FileKey))
	fmt.Printf("   Size: %s\n", formatBytes(result.SizeBytes))
	fmt.Printf("   Type: %s\n", result.ContentType)

	return nil
}

func (h *FilesHandler) Download(cmd *cobra.Command, args []string) error {
	fileKey := args[0]
	stationURL, _ := cmd.Flags().GetString("station")
	outputPath, _ := cmd.Flags().GetString("output")

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("📥 Download File"))

	fileKey = normalizeFileKey(fileKey)

	if stationURL != "" {
		return h.downloadViaAPI(fileKey, stationURL, outputPath, styles)
	}

	return h.downloadLocal(fileKey, outputPath, styles)
}

func (h *FilesHandler) downloadLocal(fileKey, outputPath string, styles CLIStyles) error {
	ctx := context.Background()

	store, cleanup, err := h.createLocalFileStore()
	if err != nil {
		return err
	}
	defer cleanup()

	reader, info, err := store.Get(ctx, fileKey)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}
	defer reader.Close()

	if outputPath == "" {
		outputPath = filepath.Base(fileKey)
		if outputPath == "." || outputPath == "/" {
			outputPath = "downloaded_file"
		}
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	n, err := io.Copy(outFile, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("%s Downloaded successfully\n", styles.Success.Render("✅"))
	fmt.Printf("   Output: %s\n", styles.Info.Render(outputPath))
	fmt.Printf("   Size: %s\n", formatBytes(n))
	fmt.Printf("   Type: %s\n", info.ContentType)

	return nil
}

func (h *FilesHandler) downloadViaAPI(fileKey, stationURL, outputPath string, styles CLIStyles) error {
	url := strings.TrimSuffix(stationURL, "/") + "/api/v1/files/" + fileKey

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if outputPath == "" {
		cd := resp.Header.Get("Content-Disposition")
		if cd != "" {
			_, params, _ := mime.ParseMediaType(cd)
			if filename, ok := params["filename"]; ok {
				outputPath = filename
			}
		}
		if outputPath == "" {
			outputPath = filepath.Base(fileKey)
			if outputPath == "." || outputPath == "/" {
				outputPath = "downloaded_file"
			}
		}
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	n, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	fmt.Printf("%s Downloaded successfully\n", styles.Success.Render("✅"))
	fmt.Printf("   Output: %s\n", styles.Info.Render(outputPath))
	fmt.Printf("   Size: %s\n", formatBytes(n))
	if contentType != "" {
		fmt.Printf("   Type: %s\n", contentType)
	}

	return nil
}

func (h *FilesHandler) List(cmd *cobra.Command, args []string) error {
	stationURL, _ := cmd.Flags().GetString("station")
	prefix, _ := cmd.Flags().GetString("prefix")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	styles := getCLIStyles(h.themeManager)

	if stationURL != "" {
		return h.listViaAPI(stationURL, prefix, jsonOutput, styles)
	}

	return h.listLocal(prefix, jsonOutput, styles)
}

func (h *FilesHandler) listLocal(prefix string, jsonOutput bool, styles CLIStyles) error {
	ctx := context.Background()

	store, cleanup, err := h.createLocalFileStore()
	if err != nil {
		return err
	}
	defer cleanup()

	files, err := store.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(files)
	}

	if !jsonOutput {
		fmt.Println(styles.Banner.Render("📁 Files"))
	}

	if len(files) == 0 {
		fmt.Println("No files found")
		return nil
	}

	fmt.Printf("Found %d file(s):\n\n", len(files))
	for _, f := range files {
		fmt.Printf("• %s\n", styles.Info.Render(f.Key))
		fmt.Printf("    Size: %s | Type: %s | Created: %s\n",
			formatBytes(f.Size),
			f.ContentType,
			f.CreatedAt.Format("2006-01-02 15:04"))
		if !f.ExpiresAt.IsZero() {
			fmt.Printf("    Expires: %s\n", f.ExpiresAt.Format("2006-01-02 15:04"))
		}
	}

	return nil
}

func (h *FilesHandler) listViaAPI(stationURL, prefix string, jsonOutput bool, styles CLIStyles) error {
	url := strings.TrimSuffix(stationURL, "/") + "/api/v1/files"
	if prefix != "" {
		url += "?prefix=" + prefix
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("list failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if jsonOutput {
		io.Copy(os.Stdout, resp.Body)
		return nil
	}

	var result struct {
		Files []struct {
			Key         string    `json:"key"`
			Size        int64     `json:"size"`
			ContentType string    `json:"content_type"`
			CreatedAt   time.Time `json:"created_at"`
			ExpiresAt   time.Time `json:"expires_at,omitempty"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Println(styles.Banner.Render("📁 Files"))

	if len(result.Files) == 0 {
		fmt.Println("No files found")
		return nil
	}

	fmt.Printf("Found %d file(s):\n\n", len(result.Files))
	for _, f := range result.Files {
		fmt.Printf("• %s\n", styles.Info.Render(f.Key))
		fmt.Printf("    Size: %s | Type: %s | Created: %s\n",
			formatBytes(f.Size),
			f.ContentType,
			f.CreatedAt.Format("2006-01-02 15:04"))
		if !f.ExpiresAt.IsZero() {
			fmt.Printf("    Expires: %s\n", f.ExpiresAt.Format("2006-01-02 15:04"))
		}
	}

	return nil
}

func (h *FilesHandler) Delete(cmd *cobra.Command, args []string) error {
	fileKey := args[0]
	stationURL, _ := cmd.Flags().GetString("station")
	force, _ := cmd.Flags().GetBool("force")

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("🗑️  Delete File"))

	fileKey = normalizeFileKey(fileKey)

	if !force {
		fmt.Printf("Delete file '%s'? [y/N]: ", fileKey)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if stationURL != "" {
		return h.deleteViaAPI(fileKey, stationURL, styles)
	}

	return h.deleteLocal(fileKey, styles)
}

func (h *FilesHandler) deleteLocal(fileKey string, styles CLIStyles) error {
	ctx := context.Background()

	store, cleanup, err := h.createLocalFileStore()
	if err != nil {
		return err
	}
	defer cleanup()

	if err := store.Delete(ctx, fileKey); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	fmt.Printf("%s Deleted: %s\n", styles.Success.Render("✅"), fileKey)
	return nil
}

func (h *FilesHandler) deleteViaAPI(fileKey, stationURL string, styles CLIStyles) error {
	url := strings.TrimSuffix(stationURL, "/") + "/api/v1/files/" + fileKey

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	fmt.Printf("%s Deleted: %s\n", styles.Success.Render("✅"), fileKey)
	return nil
}

func (h *FilesHandler) Info(cmd *cobra.Command, args []string) error {
	fileKey := args[0]
	stationURL, _ := cmd.Flags().GetString("station")

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("ℹ️  File Info"))

	fileKey = normalizeFileKey(fileKey)

	if stationURL != "" {
		return h.infoViaAPI(fileKey, stationURL, styles)
	}

	return h.infoLocal(fileKey, styles)
}

func (h *FilesHandler) infoLocal(fileKey string, styles CLIStyles) error {
	ctx := context.Background()

	store, cleanup, err := h.createLocalFileStore()
	if err != nil {
		return err
	}
	defer cleanup()

	info, err := store.GetInfo(ctx, fileKey)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	printFileInfo(info, styles)
	return nil
}

func (h *FilesHandler) infoViaAPI(fileKey, stationURL string, styles CLIStyles) error {
	url := strings.TrimSuffix(stationURL, "/") + "/api/v1/files/" + fileKey

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("file not found: %s", fileKey)
	}

	fmt.Printf("Key: %s\n", styles.Info.Render(fileKey))
	fmt.Printf("Content-Type: %s\n", resp.Header.Get("Content-Type"))
	fmt.Printf("Content-Length: %s\n", resp.Header.Get("Content-Length"))

	if checksum := resp.Header.Get("X-Checksum"); checksum != "" {
		fmt.Printf("Checksum: %s\n", checksum)
	}
	if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
		fmt.Printf("Last-Modified: %s\n", lastMod)
	}

	return nil
}

func (h *FilesHandler) createLocalFileStore() (storage.FileStore, func(), error) {
	natsURL := os.Getenv("WORKFLOW_NATS_URL")
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:4222"
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to NATS at %s: %w", natsURL, err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("failed to get JetStream context: %w", err)
	}

	store, err := storage.NewNATSFileStore(js, storage.DefaultConfig())
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("failed to create file store: %w", err)
	}

	cleanup := func() {
		store.Close()
		nc.Close()
	}

	return store, cleanup, nil
}

func normalizeFileKey(key string) string {
	if strings.HasPrefix(key, "f_") && !strings.Contains(key, "/") {
		return storage.GenerateUserFileKey(key)
	}
	return key
}

func detectContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeTypes := map[string]string{
		".csv":  "text/csv",
		".json": "application/json",
		".xml":  "application/xml",
		".txt":  "text/plain",
		".html": "text/html",
		".htm":  "text/html",
		".pdf":  "application/pdf",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".zip":  "application/zip",
		".gz":   "application/gzip",
		".tar":  "application/x-tar",
		".yaml": "application/yaml",
		".yml":  "application/yaml",
		".md":   "text/markdown",
		".py":   "text/x-python",
		".go":   "text/x-go",
		".js":   "text/javascript",
		".ts":   "text/typescript",
	}

	if contentType, ok := mimeTypes[ext]; ok {
		return contentType
	}
	return "application/octet-stream"
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func printFileInfo(info *storage.FileInfo, styles CLIStyles) {
	fmt.Printf("Key: %s\n", styles.Info.Render(info.Key))
	fmt.Printf("Size: %s\n", formatBytes(info.Size))
	fmt.Printf("Content-Type: %s\n", info.ContentType)
	if info.Checksum != "" {
		fmt.Printf("Checksum: %s\n", info.Checksum)
	}
	fmt.Printf("Created: %s\n", info.CreatedAt.Format(time.RFC3339))
	if !info.ExpiresAt.IsZero() {
		fmt.Printf("Expires: %s\n", info.ExpiresAt.Format(time.RFC3339))
	}
	if len(info.Metadata) > 0 {
		fmt.Println("Metadata:")
		for k, v := range info.Metadata {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
}
