//go:build integration

package blobstore

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
)

// liveCfg builds a Config from BLOBSTORE_* env, skipping if MinIO isn't wired.
func liveCfg(t *testing.T) Config {
	t.Helper()
	if os.Getenv("BLOBSTORE_ENDPOINT") == "" {
		t.Fatal("BLOBSTORE_ENDPOINT not set — run against the dev MinIO")
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

func TestMinIOStore_RoundTrip(t *testing.T) {
	cfg := liveCfg(t)
	ctx := context.Background()
	s, err := NewMinIOStore(ctx, cfg)
	if err != nil {
		t.Fatalf("NewMinIOStore: %v", err)
	}
	want := []byte("live transcript bytes\n")
	ref, err := s.Put(ctx, "test/round-trip.txt", bytes.NewReader(want))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, err := s.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, want) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, want)
	}
}
