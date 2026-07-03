// Package blobstore is the Tier-0 object-storage seam: opaque blob bytes keyed
// by a caller-chosen key, returning a stable ref for later retrieval. The
// interface is the contract; MinIO/S3 is the dev/prod backend (adapter in
// minio.go). No domain imports — a leaf.
package blobstore

import (
	"context"
	"io"
)

// Store persists opaque blobs. Put stores r under key and returns a stable ref
// the same backend resolves via Get. ref may equal key (memory/MinIO) but
// callers must treat it as opaque and store it verbatim.
type Store interface {
	Put(ctx context.Context, key string, r io.Reader) (ref string, err error)
	Get(ctx context.Context, ref string) (io.ReadCloser, error)
}
