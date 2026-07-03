package keyvault_test

import (
	"encoding/base64"
	"testing"

	"github.com/gradionhq/margince/backend/internal/platform/keyvault"
)

func TestLoadConfig_MissingKey(t *testing.T) {
	t.Setenv("KEYVAULT_MASTER_KEY", "")
	if _, err := keyvault.LoadConfig(); err == nil {
		t.Fatalf("LoadConfig must fail when KEYVAULT_MASTER_KEY is unset")
	}
}

func TestLoadConfig_WrongLength(t *testing.T) {
	t.Setenv("KEYVAULT_MASTER_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16)))
	if _, err := keyvault.LoadConfig(); err == nil {
		t.Fatalf("LoadConfig must fail for a non-32-byte decoded key")
	}
}

func TestLoadConfig_Valid(t *testing.T) {
	raw := make([]byte, 32)
	t.Setenv("KEYVAULT_MASTER_KEY", base64.StdEncoding.EncodeToString(raw))
	cfg, err := keyvault.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.MasterKey) != 32 {
		t.Fatalf("cfg.MasterKey len = %d, want 32", len(cfg.MasterKey))
	}
}
