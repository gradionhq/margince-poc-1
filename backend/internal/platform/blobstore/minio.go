package blobstore

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOStore is the S3/MinIO-backed Store. Put/Get target a single bucket; the
// ref is the object key. Prod points Config at real S3 with no code change.
type MinIOStore struct {
	client *minio.Client
	bucket string
}

// NewMinIOStore dials the endpoint in cfg and ensures cfg.Bucket exists.
func NewMinIOStore(ctx context.Context, cfg Config) (*MinIOStore, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("blobstore: dial minio: %w", err)
	}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("blobstore: bucket check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("blobstore: make bucket %q: %w", cfg.Bucket, err)
		}
	}
	return &MinIOStore{client: client, bucket: cfg.Bucket}, nil
}

// Put streams r to the object named key and returns key as the ref.
func (s *MinIOStore) Put(ctx context.Context, key string, r io.Reader) (string, error) {
	if _, err := s.client.PutObject(ctx, s.bucket, key, r, -1, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	}); err != nil {
		return "", fmt.Errorf("blobstore: put %q: %w", key, err)
	}
	return key, nil
}

// Get opens the object named ref for reading.
func (s *MinIOStore) Get(ctx context.Context, ref string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, ref, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("blobstore: get %q: %w", ref, err)
	}
	// GetObject is lazy; force a stat so a missing object errors here, not on first Read.
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("blobstore: stat %q: %w", ref, err)
	}
	return obj, nil
}

func (s *MinIOStore) PresignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", fmt.Errorf("blobstore: presigned put %s: %w", key, err)
	}
	return u.String(), nil
}

func (s *MinIOStore) PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, url.Values{})
	if err != nil {
		return "", fmt.Errorf("blobstore: presigned get %s: %w", key, err)
	}
	return u.String(), nil
}
