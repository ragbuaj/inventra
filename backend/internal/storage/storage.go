package storage

import (
	"context"
	"errors"
	"io"
)

// ErrObjectNotFound is returned by Get/Remove when the key does not exist.
var ErrObjectNotFound = errors.New("object not found")

// ObjectInfo carries the minimal metadata needed to serve an object.
type ObjectInfo struct {
	ContentType string
	Size        int64
}

// Storage is an S3-compatible object store abstraction.
type Storage interface {
	EnsureBucket(ctx context.Context) error
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Remove(ctx context.Context, key string) error
}
