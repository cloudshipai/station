# PRD: Sandbox File Staging with NATS Object Store

**Status**: Complete ✅  
**Author**: Station Team  
**Date**: 2025-12-30  
**Completed**: 2025-12-30  
**Version**: 1.0

---

## Problem Statement

Station's sandbox system allows agents to run code in isolated Docker containers with persistent workspaces across workflow steps. However, there's a critical gap in the file I/O story:

| Operation | Current State | Problem |
|-----------|--------------|---------|
| **Agent writes file** | `sandbox_fs_write` | Works, but requires passing content through LLM context |
| **Agent reads file** | `sandbox_fs_read` | Works, but returns content through LLM context |
| **User uploads input file** | N/A | No way to inject external files into sandbox |
| **User downloads output file** | N/A | No way to extract files from sandbox |
| **Large files** | N/A | LLM context window limits make large files impossible |

### Current Workaround

```yaml
# User has to paste CSV content into the prompt
task: |
  Here's my CSV data:
  id,name,value
  1,foo,100
  2,bar,200
  ... (thousands of rows)
  
  Process this and output the result.
```

This doesn't scale:
- LLM context windows have limits (128K-200K tokens)
- Token costs increase with file size
- Binary files are impossible
- Multi-MB files are impractical

---

## Solution: NATS Object Store File Staging

Use NATS JetStream Object Store as a file staging area between users and sandboxes.

### Architecture

```
                                    ┌─────────────────────────────┐
                                    │       NATS JetStream        │
                                    │   Object Store: "files"     │
                                    │                             │
                                    │  files/{file_id}            │
                                    │  runs/{run_id}/output/*     │
                                    │  sessions/{session_id}/*    │
                                    └──────────────┬──────────────┘
                                                   │
                 ┌─────────────────────────────────┼─────────────────────────────────┐
                 │                                 │                                 │
                 ▼                                 ▼                                 ▼
        ┌────────────────┐               ┌────────────────┐               ┌────────────────┐
        │   User/CLI     │               │     Agent      │               │     API        │
        │                │               │                │               │                │
        │ stn files      │               │ sandbox_       │               │ POST /files    │
        │   upload       │───────────────│   stage_file   │───────────────│ GET  /files    │
        │   download     │               │   publish_file │               │ DELETE /files  │
        └────────────────┘               └───────┬────────┘               └────────────────┘
                                                 │
                                                 ▼
                                        ┌────────────────┐
                                        │    Sandbox     │
                                        │   Container    │
                                        │                │
                                        │  /workspace/   │
                                        │    input/      │
                                        │    output/     │
                                        └────────────────┘
```

### Flow Example

```bash
# 1. User uploads input file
$ stn files upload data.csv
Uploaded: files/f_abc123 (2.4 MB)

# 2. User starts workflow with file reference
$ stn workflow run csv-pipeline --input '{"input_file": "files/f_abc123"}'

# 3. Agent stages file into sandbox
# (tool call by LLM)
sandbox_stage_file(file_key="files/f_abc123", destination="input/data.csv")
# → Fetches from NATS Object Store → Writes to sandbox /workspace/input/data.csv

# 4. Agent processes file in sandbox
sandbox_exec(cmd=["python", "transform.py", "input/data.csv", "output/result.csv"])

# 5. Agent publishes output file
sandbox_publish_file(source="output/result.csv")
# → Reads from sandbox → Uploads to NATS Object Store
# → Returns: files/f_xyz789

# 6. User downloads result
$ stn files download f_xyz789 -o result.csv
Downloaded: result.csv (1.8 MB)
```

---

## File Key Convention

```
sandbox-files/                          # Object Store bucket name
├── files/{file_id}                     # User-uploaded files (permanent until deleted)
├── runs/{run_id}/output/{filename}     # Workflow run outputs (auto-cleanup after TTL)
└── sessions/{session_id}/{filename}    # Session artifacts (cleanup with session)
```

### File ID Format

- **User uploads**: `f_{ulid}` (e.g., `f_01JGXYZ123ABC`)
- **Run outputs**: `runs/{run_id}/output/{original_filename}`
- **Session files**: `sessions/{session_id}/{path}`

---

## Implementation Plan

### Phase 1: NATS Object Store Wrapper

**File**: `internal/storage/file_store.go`

```go
package storage

type FileStore interface {
    // Put stores a file, returns the file key
    Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) (*FileInfo, error)
    
    // Get retrieves a file by key
    Get(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error)
    
    // Delete removes a file
    Delete(ctx context.Context, key string) error
    
    // List files with optional prefix
    List(ctx context.Context, prefix string) ([]*FileInfo, error)
    
    // Exists checks if a file exists
    Exists(ctx context.Context, key string) (bool, error)
}

type FileInfo struct {
    Key         string
    Size        int64
    ContentType string
    Checksum    string    // SHA-256
    CreatedAt   time.Time
    Metadata    map[string]string
}

type PutOptions struct {
    ContentType string
    Metadata    map[string]string
    TTL         time.Duration  // 0 = no expiration
}
```

### Phase 2: Engine Integration

**Modify**: `internal/workflows/runtime/nats_engine.go`

Add Object Store bucket creation in `NewEngine()`:

```go
// Create Object Store bucket for file staging
objConfig := &nats.ObjectStoreConfig{
    Bucket:      "sandbox-files",
    Description: "File staging for sandbox operations",
    Storage:     storageType,
    MaxBytes:    10 * 1024 * 1024 * 1024, // 10 GB default
}
_, err = js.CreateObjectStore(objConfig)
```

### Phase 3: Sandbox Tools

**Modify**: `internal/services/sandbox_code_tools.go`

Add two new tools:

#### `sandbox_stage_file`

```go
type SandboxStageFileInput struct {
    SandboxID   string `json:"sandbox_id"`
    FileKey     string `json:"file_key"`      // Key in NATS Object Store
    Destination string `json:"destination"`   // Path in sandbox (relative to /workspace)
}

type SandboxStageFileOutput struct {
    OK          bool   `json:"ok"`
    Path        string `json:"path"`
    SizeBytes   int64  `json:"size_bytes"`
}
```

Implementation:
1. Validate sandbox exists
2. Fetch file from NATS Object Store using `FileStore.Get()`
3. Write to sandbox using `backend.WriteFile()`
4. Return success with file info

#### `sandbox_publish_file`

```go
type SandboxPublishFileInput struct {
    SandboxID string `json:"sandbox_id"`
    Source    string `json:"source"`         // Path in sandbox (relative to /workspace)
    FileKey   string `json:"file_key,omitempty"` // Optional key override
}

type SandboxPublishFileOutput struct {
    OK      bool   `json:"ok"`
    FileKey string `json:"file_key"`
    SizeBytes int64 `json:"size_bytes"`
}
```

Implementation:
1. Validate sandbox exists
2. Read file from sandbox using `backend.ReadFile()`
3. Upload to NATS Object Store using `FileStore.Put()`
4. Return the file key for later retrieval

### Phase 4: API Endpoints

**File**: `internal/api/v1/files.go`

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/files` | Upload file (multipart) |
| `GET` | `/api/v1/files/{key}` | Download file |
| `GET` | `/api/v1/files` | List files with prefix filter |
| `DELETE` | `/api/v1/files/{key}` | Delete file |
| `HEAD` | `/api/v1/files/{key}` | Get file metadata |

#### Upload Request

```http
POST /api/v1/files HTTP/1.1
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="file"; filename="data.csv"
Content-Type: text/csv

id,name,value
1,foo,100
...
--boundary--
```

Response:
```json
{
  "file_key": "files/f_01JGXYZ123ABC",
  "size_bytes": 2457600,
  "content_type": "text/csv",
  "checksum": "sha256:abc123..."
}
```

### Phase 5: CLI Commands

**File**: `cmd/main/files.go`

```bash
# Upload file
stn files upload <path> [--key <custom_key>] [--ttl <duration>]

# Download file
stn files download <file_key> [-o <output_path>]

# List files
stn files list [--prefix <prefix>] [--json]

# Delete file
stn files delete <file_key> [--force]

# Remote operations (via Station API)
stn files upload <path> --station <station_url>
stn files download <file_key> --station <station_url> -o <output>
```

---

## Configuration

### Station Config (`config.yaml`)

```yaml
storage:
  # Object Store bucket name
  bucket: "sandbox-files"
  
  # Maximum file size (default: 100 MB)
  max_file_size: 104857600
  
  # Default TTL for uploaded files (0 = no expiration)
  default_ttl: 0
  
  # Maximum total storage (default: 10 GB)
  max_total_bytes: 10737418240
  
  # Cleanup settings
  cleanup:
    # Enable automatic cleanup of expired files
    enabled: true
    # Cleanup interval
    interval: 1h
```

---

## Security Considerations

1. **File Size Limits**: Enforce maximum file size at upload
2. **Content Type Validation**: Validate MIME types for security
3. **Path Traversal**: Sanitize destination paths in `sandbox_stage_file`
4. **Quota Management**: Track storage usage per organization/user
5. **Checksum Verification**: SHA-256 checksums for integrity

---

## Error Handling

| Error | HTTP Code | Tool Error |
|-------|-----------|------------|
| File not found | 404 | `file not found: {key}` |
| File too large | 413 | `file exceeds maximum size of {max}` |
| Storage quota exceeded | 507 | `storage quota exceeded` |
| Invalid file key | 400 | `invalid file key format` |
| Sandbox not found | 404 | `sandbox not found: {id}` |

---

## Testing Strategy

### Unit Tests

1. `internal/storage/file_store_test.go` - FileStore operations
2. `internal/services/sandbox_stage_file_test.go` - Stage tool
3. `internal/services/sandbox_publish_file_test.go` - Publish tool
4. `internal/api/v1/files_test.go` - API endpoints

### Integration Tests

```go
func TestFileStagingE2E(t *testing.T) {
    // 1. Upload file via API
    // 2. Stage file into sandbox
    // 3. Process file in sandbox
    // 4. Publish output
    // 5. Download via API
    // 6. Verify content
}
```

---

## Implementation Status

### Phase 1: NATS Object Store Wrapper ✅
- [x] Create `internal/storage/file_store.go` - FileStore interface, FileInfo, PutOptions, Config
- [x] Create `internal/storage/nats_file_store.go` - NATSFileStore implementation
- [x] Create `internal/storage/errors.go` - Error types (ErrFileNotFound, ErrFileTooLarge, etc.)
- [x] Create `internal/storage/ulid.go` - ULID generation for file IDs
- [x] Write unit tests - 10 tests passing

### Phase 2: Engine Integration ✅
- [x] Add `JetStream()` method to Engine interface
- [x] Expose JetStream context in NATSEngine for FileStore creation

### Phase 3: Sandbox Tools ✅
- [x] Add `sandbox_stage_file` tool - fetches from NATS OS, writes to sandbox
- [x] Add `sandbox_publish_file` tool - reads from sandbox, uploads to NATS OS
- [x] Inject FileStore dependency into CodeModeToolFactory
- [x] Add `SetFileStore` method to AgentServiceInterface and implementations
- [x] Wire FileStore from API handlers to AgentService → ExecutionEngine → SandboxFactory
- [ ] Write unit tests (deferred)

### Phase 4: API Endpoints ✅
- [x] Create `internal/api/v1/files.go` - FilesHandler with all endpoints
- [x] Wire FilesHandler into APIHandlers via `initFilesHandler()`
- [x] Register routes in `RegisterRoutes()`
- [x] File upload endpoint: `POST /api/v1/files`
- [x] File download endpoint: `GET /api/v1/files/:key`
- [x] File list endpoint: `GET /api/v1/files`
- [x] File metadata endpoint: `HEAD /api/v1/files/:key`
- [x] File delete endpoint: `DELETE /api/v1/files/:key`

### Phase 5: CLI Commands ✅
- [x] Create `stn files upload` - upload local file to NATS Object Store
- [x] Create `stn files download` - download file from store
- [x] Create `stn files list` - list files with optional prefix filter
- [x] Create `stn files delete` - delete file with confirmation
- [x] Create `stn files info` - show file metadata
- [x] Add `--station` flag for remote operations via HTTP API

---

## Dependencies

- NATS JetStream with Object Store (already included in Station)
- `github.com/oklog/ulid/v2` for file ID generation

---

## Future Enhancements

1. **Streaming uploads/downloads** for very large files
2. **Resumable uploads** with chunking
3. **File versioning** for collaboration
4. **Pre-signed URLs** for direct client-to-NATS transfers
5. **CloudShip integration** for cloud-hosted file storage
