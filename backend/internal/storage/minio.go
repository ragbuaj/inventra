package storage

import (
	"context"
	"errors"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOStorage is a Storage backed by a MinIO (or S3-compatible) server.
type MinIOStorage struct {
	client *minio.Client
	bucket string
}

// NewMinIOStorage creates a MinIOStorage and returns it; the bucket is not created here —
// call EnsureBucket after construction before first use.
func NewMinIOStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStorage, error) {
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	return &MinIOStorage{client: c, bucket: bucket}, nil
}

// EnsureBucket creates the configured bucket if it does not already exist.
func (s *MinIOStorage) EnsureBucket(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}

// Put uploads r under key with the given content-type.
func (s *MinIOStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

// Get retrieves the object at key. Returns ErrObjectNotFound when the key does not exist.
func (s *MinIOStorage) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		var resp minio.ErrorResponse
		if errors.As(err, &resp) && resp.Code == "NoSuchKey" {
			return nil, ObjectInfo{}, ErrObjectNotFound
		}
		return nil, ObjectInfo{}, err
	}
	return obj, ObjectInfo{ContentType: info.ContentType, Size: info.Size}, nil
}

// Remove deletes the object at key. It is not an error if the key does not exist.
func (s *MinIOStorage) Remove(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
