package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

type NATSFileStore struct {
	js     nats.JetStreamContext
	bucket nats.ObjectStore
	config Config
}

func NewNATSFileStore(js nats.JetStreamContext, config Config) (*NATSFileStore, error) {
	if js == nil {
		return nil, ErrStoreNotInitialized
	}

	if config.BucketName == "" {
		config = DefaultConfig()
	}

	bucket, err := js.ObjectStore(config.BucketName)
	if err != nil {
		if err == nats.ErrBucketNotFound || err == nats.ErrStreamNotFound || strings.Contains(err.Error(), "stream not found") {
			bucket, err = js.CreateObjectStore(&nats.ObjectStoreConfig{
				Bucket:      config.BucketName,
				Description: "File staging for sandbox operations",
				MaxBytes:    config.MaxTotalBytes,
			})
			if err != nil {
				return nil, NewFileError("create_bucket", config.BucketName, err)
			}
		} else {
			return nil, NewFileError("get_bucket", config.BucketName, err)
		}
	}

	return &NATSFileStore{
		js:     js,
		bucket: bucket,
		config: config,
	}, nil
}

func (s *NATSFileStore) Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) (*FileInfo, error) {
	if s.bucket == nil {
		return nil, ErrStoreNotInitialized
	}

	if key == "" {
		return nil, NewFileError("put", "", ErrInvalidKey)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, NewFileError("put", key, fmt.Errorf("read input: %w", err))
	}

	if s.config.MaxFileSize > 0 && int64(len(data)) > s.config.MaxFileSize {
		return nil, NewFileError("put", key, ErrFileTooLarge)
	}

	checksum := sha256.Sum256(data)
	checksumHex := hex.EncodeToString(checksum[:])

	meta := &nats.ObjectMeta{
		Name:        key,
		Description: opts.Description,
		Headers:     make(nats.Header),
	}

	if opts.ContentType != "" {
		meta.Headers.Set("Content-Type", opts.ContentType)
	}
	meta.Headers.Set("X-Checksum", checksumHex)

	if opts.TTL > 0 {
		expiresAt := time.Now().Add(opts.TTL)
		meta.Headers.Set("X-Expires-At", expiresAt.Format(time.RFC3339))
	}

	for k, v := range opts.Metadata {
		meta.Headers.Set("X-Meta-"+k, v)
	}

	_, err = s.bucket.Put(meta, bytes.NewReader(data))
	if err != nil {
		if strings.Contains(err.Error(), "maximum bytes") {
			return nil, NewFileError("put", key, ErrStorageQuotaExceeded)
		}
		return nil, NewFileError("put", key, err)
	}

	info := &FileInfo{
		Key:         key,
		Size:        int64(len(data)),
		ContentType: opts.ContentType,
		Checksum:    checksumHex,
		CreatedAt:   time.Now(),
		Metadata:    opts.Metadata,
	}

	if opts.TTL > 0 {
		info.ExpiresAt = time.Now().Add(opts.TTL)
	}

	return info, nil
}

func (s *NATSFileStore) Get(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error) {
	if s.bucket == nil {
		return nil, nil, ErrStoreNotInitialized
	}

	if key == "" {
		return nil, nil, NewFileError("get", "", ErrInvalidKey)
	}

	result, err := s.bucket.Get(key)
	if err != nil {
		if err == nats.ErrObjectNotFound {
			return nil, nil, NewFileError("get", key, ErrFileNotFound)
		}
		return nil, nil, NewFileError("get", key, err)
	}

	objInfo, err := result.Info()
	if err != nil {
		result.Close()
		return nil, nil, NewFileError("get", key, err)
	}

	info := s.objectInfoToFileInfo(objInfo)

	return result, info, nil
}

func (s *NATSFileStore) Delete(ctx context.Context, key string) error {
	if s.bucket == nil {
		return ErrStoreNotInitialized
	}

	if key == "" {
		return NewFileError("delete", "", ErrInvalidKey)
	}

	err := s.bucket.Delete(key)
	if err != nil {
		if err == nats.ErrObjectNotFound {
			return NewFileError("delete", key, ErrFileNotFound)
		}
		return NewFileError("delete", key, err)
	}

	return nil
}

func (s *NATSFileStore) List(ctx context.Context, prefix string) ([]*FileInfo, error) {
	if s.bucket == nil {
		return nil, ErrStoreNotInitialized
	}

	objects, err := s.bucket.List()
	if err != nil {
		return nil, NewFileError("list", prefix, err)
	}

	var files []*FileInfo
	for _, obj := range objects {
		if prefix == "" || strings.HasPrefix(obj.Name, prefix) {
			files = append(files, s.objectInfoToFileInfo(obj))
		}
	}

	return files, nil
}

func (s *NATSFileStore) Exists(ctx context.Context, key string) (bool, error) {
	if s.bucket == nil {
		return false, ErrStoreNotInitialized
	}

	if key == "" {
		return false, NewFileError("exists", "", ErrInvalidKey)
	}

	_, err := s.bucket.GetInfo(key)
	if err != nil {
		if err == nats.ErrObjectNotFound {
			return false, nil
		}
		return false, NewFileError("exists", key, err)
	}

	return true, nil
}

func (s *NATSFileStore) GetInfo(ctx context.Context, key string) (*FileInfo, error) {
	if s.bucket == nil {
		return nil, ErrStoreNotInitialized
	}

	if key == "" {
		return nil, NewFileError("get_info", "", ErrInvalidKey)
	}

	objInfo, err := s.bucket.GetInfo(key)
	if err != nil {
		if err == nats.ErrObjectNotFound {
			return nil, NewFileError("get_info", key, ErrFileNotFound)
		}
		return nil, NewFileError("get_info", key, err)
	}

	return s.objectInfoToFileInfo(objInfo), nil
}

func (s *NATSFileStore) Close() error {
	return nil
}

func (s *NATSFileStore) objectInfoToFileInfo(obj *nats.ObjectInfo) *FileInfo {
	info := &FileInfo{
		Key:       obj.Name,
		Size:      int64(obj.Size),
		CreatedAt: obj.ModTime,
		Metadata:  make(map[string]string),
	}

	if obj.Headers != nil {
		info.ContentType = obj.Headers.Get("Content-Type")
		info.Checksum = obj.Headers.Get("X-Checksum")

		if expiresStr := obj.Headers.Get("X-Expires-At"); expiresStr != "" {
			if t, err := time.Parse(time.RFC3339, expiresStr); err == nil {
				info.ExpiresAt = t
			}
		}

		for key, values := range obj.Headers {
			if strings.HasPrefix(key, "X-Meta-") && len(values) > 0 {
				metaKey := strings.TrimPrefix(key, "X-Meta-")
				info.Metadata[metaKey] = values[0]
			}
		}
	}

	return info
}

func (s *NATSFileStore) CleanupExpired(ctx context.Context) (int, error) {
	if s.bucket == nil {
		return 0, ErrStoreNotInitialized
	}

	objects, err := s.bucket.List()
	if err != nil {
		return 0, NewFileError("cleanup", "", err)
	}

	now := time.Now()
	var deleted int

	for _, obj := range objects {
		if obj.Headers != nil {
			if expiresStr := obj.Headers.Get("X-Expires-At"); expiresStr != "" {
				if t, err := time.Parse(time.RFC3339, expiresStr); err == nil && now.After(t) {
					if err := s.bucket.Delete(obj.Name); err == nil {
						deleted++
					}
				}
			}
		}
	}

	return deleted, nil
}
