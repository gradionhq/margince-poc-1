package blobstore

import (
	"bytes"
	"context"
	"io"
	"testing"
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
