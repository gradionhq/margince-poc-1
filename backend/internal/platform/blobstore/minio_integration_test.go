//go:build integration

package blobstore

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
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

func TestMinIOStore_PresignedPutThenGet_RoundTripsRealBytes(t *testing.T) {
	cfg := liveCfg(t)
	ctx := context.Background()
	s, err := NewMinIOStore(ctx, cfg)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	key := "attachments/it-test/presign-roundtrip.txt"
	putURL, err := s.PresignedPutURL(ctx, key, 5*time.Minute)
	if err != nil {
		t.Fatalf("presigned put url: %v", err)
	}
	body := []byte("presigned bytes never touch the app process")
	req, err := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build put request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do put: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("expected 2xx from presigned PUT, got %d", resp.StatusCode)
	}

	getURL, err := s.PresignedGetURL(ctx, key, 5*time.Minute)
	if err != nil {
		t.Fatalf("presigned get url: %v", err)
	}
	getResp, err := http.Get(getURL)
	if err != nil {
		t.Fatalf("do get: %v", err)
	}
	defer getResp.Body.Close()
	got, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("expected round-tripped bytes %q, got %q", body, got)
	}
}
