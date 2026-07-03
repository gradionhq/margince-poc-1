// This file holds keyvault's own env-reading seam (mirrors
// blobstore.Config/LoadConfig). Reading KEYVAULT_MASTER_KEY here does not
// breach the one-env-root-per-binary rule: this is the seam's own contract,
// called only from cmd/server's loadConfig, which composes it alongside the
// binary's other env reads as a single root.

package keyvault

import (
	"encoding/base64"
	"fmt"
	"os"
)

// Config holds the decoded 32-byte AES-256 master key.
type Config struct {
	MasterKey []byte
}

// LoadConfig reads KEYVAULT_MASTER_KEY (base64, 32 raw bytes) from the
// environment. Required — returns an error if unset, malformed, or the
// wrong length once decoded.
func LoadConfig() (Config, error) {
	raw := os.Getenv("KEYVAULT_MASTER_KEY")
	if raw == "" {
		return Config{}, fmt.Errorf("keyvault: KEYVAULT_MASTER_KEY is required")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return Config{}, fmt.Errorf("keyvault: KEYVAULT_MASTER_KEY: invalid base64: %w", err)
	}
	if len(key) != 32 {
		return Config{}, fmt.Errorf("keyvault: KEYVAULT_MASTER_KEY: decoded length must be 32 bytes, got %d", len(key))
	}
	return Config{MasterKey: key}, nil
}
