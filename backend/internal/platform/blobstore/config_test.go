package blobstore

import "testing"

func TestLoadConfig_RequiresFields(t *testing.T) {
	t.Setenv("BLOBSTORE_ENDPOINT", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig with empty endpoint should error")
	}
}

func TestLoadConfig_ReadsEnv(t *testing.T) {
	t.Setenv("BLOBSTORE_ENDPOINT", "localhost:9000")
	t.Setenv("BLOBSTORE_BUCKET", "transcripts")
	t.Setenv("BLOBSTORE_ACCESS_KEY", "minioadmin")
	t.Setenv("BLOBSTORE_SECRET_KEY", "minioadmin")
	t.Setenv("BLOBSTORE_USE_SSL", "false")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Endpoint != "localhost:9000" || cfg.Bucket != "transcripts" || cfg.UseSSL {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
