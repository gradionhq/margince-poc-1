package blobstore

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestMemoryStore_PutGetRoundTrip(t *testing.T) {
	var s Store = NewMemoryStore()
	ctx := context.Background()
	want := []byte("hello transcript\nline two\n")

	ref, err := s.Put(ctx, "ws1/meeting/evt-123", bytes.NewReader(want))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if ref == "" {
		t.Fatal("Put returned empty ref")
	}

	rc, err := s.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("round-trip mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestMemoryStore_GetMissing(t *testing.T) {
	s := NewMemoryStore()
	if _, err := s.Get(context.Background(), "nope"); err == nil {
		t.Fatal("Get(missing) should error")
	}
}

func TestMemoryStore_PresignedPutURL_ReturnsDeterministicURL(t *testing.T) {
	m := NewMemoryStore()
	u, err := m.PresignedPutURL(context.Background(), "attachments/ws1/att1/f.pdf", 15*time.Minute)
	if err != nil {
		t.Fatalf("presign put: %v", err)
	}
	if !strings.HasPrefix(u, "memory://attachments/ws1/att1/f.pdf") {
		t.Fatalf("expected memory:// URL prefixed with the key, got %q", u)
	}
}

func TestMemoryStore_PresignedGetURL_MatchesPutRoundTrip(t *testing.T) {
	m := NewMemoryStore()
	if _, err := m.Put(context.Background(), "k1", strings.NewReader("hello")); err != nil {
		t.Fatalf("put: %v", err)
	}
	u, err := m.PresignedGetURL(context.Background(), "k1", 5*time.Minute)
	if err != nil {
		t.Fatalf("presign get: %v", err)
	}
	if !strings.HasPrefix(u, "memory://k1") {
		t.Fatalf("expected memory:// URL, got %q", u)
	}
}
