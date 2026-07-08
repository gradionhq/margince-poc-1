package blobstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// MemoryStore is an in-process Store for unit tests and local dev. ref == key.
type MemoryStore struct {
	mu   sync.RWMutex
	blob map[string][]byte
}

// NewMemoryStore returns an empty in-memory Store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{blob: map[string][]byte{}} }

// Put copies r's bytes under key and returns key as the ref.
func (m *MemoryStore) Put(_ context.Context, key string, r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("blobstore: read: %w", err)
	}
	m.mu.Lock()
	m.blob[key] = b
	m.mu.Unlock()
	return key, nil
}

// Get returns the bytes stored under ref, or an error if absent.
func (m *MemoryStore) Get(_ context.Context, ref string) (io.ReadCloser, error) {
	m.mu.RLock()
	b, ok := m.blob[ref]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("blobstore: ref %q not found", ref)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

// PresignedPutURL returns a synthetic memory:// URL — MemoryStore is a
// dev/test fake with no real HTTP server behind it, so this has no live
// upload semantics; it exists only so callers exercising the Store interface
// (e.g. handler tests) get a stable, inspectable value instead of an error.
func (m *MemoryStore) PresignedPutURL(_ context.Context, key string, expiry time.Duration) (string, error) {
	return fmt.Sprintf("memory://%s?op=put&expires=%d", key, time.Now().Add(expiry).Unix()), nil
}

func (m *MemoryStore) PresignedGetURL(_ context.Context, key string, expiry time.Duration) (string, error) {
	return fmt.Sprintf("memory://%s?op=get&expires=%d", key, time.Now().Add(expiry).Unix()), nil
}
