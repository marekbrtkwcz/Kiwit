package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// Storage is the abstraction over attachment file backends (local disk, S3/MinIO).
type Storage interface {
	// Put stores data under key and returns the storage path/key to persist in DB.
	Put(ctx context.Context, key string, r io.Reader, size int64) (string, error)
	// Open returns a reader for the stored object.
	Open(ctx context.Context, path string) (io.ReadCloser, error)
	// Delete removes the stored object. Missing objects are not an error.
	Delete(ctx context.Context, path string) error
}

// LocalStorage stores files on the local filesystem under dir.
type LocalStorage struct {
	dir string
}

func NewLocal(dir string) *LocalStorage {
	return &LocalStorage{dir: dir}
}

func (s *LocalStorage) Put(_ context.Context, key string, r io.Reader, _ int64) (string, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(s.dir, filepath.Base(key))
	dst, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer func() { _ = dst.Close() }()
	if _, err := io.Copy(dst, r); err != nil {
		_ = os.Remove(p)
		return "", err
	}
	return p, nil
}

func (s *LocalStorage) Open(_ context.Context, path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (s *LocalStorage) Delete(_ context.Context, path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
