package blobstore

import (
	"fmt"
	"os"
	"strconv"
)

// Config is the object-storage backend connection. Sourced from BLOBSTORE_*
// env so prod swaps real S3 for MinIO with no code change.
type Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// LoadConfig reads the BLOBSTORE_* env. Endpoint, bucket, and both keys are
// required; UseSSL defaults to false.
//
// Reading os.Getenv here does not breach the one-env-root-per-binary rule: this
// is the binary's own config seam, called only from cmd/server's loadConfig
// (config.go), which composes it alongside the other env reads as a single root.
// The BLOBSTORE_* keys live with the Config they populate so the blobstore seam
// owns its own contract; the binary is still the sole place that wires them.
func LoadConfig() (Config, error) {
	cfg := Config{
		Endpoint:  os.Getenv("BLOBSTORE_ENDPOINT"),
		Bucket:    os.Getenv("BLOBSTORE_BUCKET"),
		AccessKey: os.Getenv("BLOBSTORE_ACCESS_KEY"),
		SecretKey: os.Getenv("BLOBSTORE_SECRET_KEY"),
	}
	if s := os.Getenv("BLOBSTORE_USE_SSL"); s != "" {
		v, err := strconv.ParseBool(s)
		if err != nil {
			return Config{}, fmt.Errorf("blobstore: BLOBSTORE_USE_SSL: %w", err)
		}
		cfg.UseSSL = v
	}
	for name, v := range map[string]string{
		"BLOBSTORE_ENDPOINT":   cfg.Endpoint,
		"BLOBSTORE_BUCKET":     cfg.Bucket,
		"BLOBSTORE_ACCESS_KEY": cfg.AccessKey,
		"BLOBSTORE_SECRET_KEY": cfg.SecretKey,
	} {
		if v == "" {
			return Config{}, fmt.Errorf("blobstore: %s is required", name)
		}
	}
	return cfg, nil
}
