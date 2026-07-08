// Package config is the single composition root for server environment resolution.
// Every env var the cmd/api binary reads is resolved here, once, at startup.
// No other file in cmd/api may call os.Getenv — thread values through Config.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gradionhq/margince/backend/internal/platform/blobstore"
	"github.com/gradionhq/margince/backend/internal/platform/keyvault"
)

// Config holds all runtime configuration for the server.
type Config struct {
	// DatabaseURL is the Postgres DSN. Required.
	DatabaseURL string

	// RedisURL is the Redis connection URL for the outbox relay and AI result cache.
	// Empty disables both features gracefully.
	RedisURL string

	// Addr is the HTTP listen address. Default: ":8080".
	Addr string

	// AIRoutingConfig is the path to ai-routing.yaml.
	// Empty disables the AI runtime; core CRUD is unaffected.
	AIRoutingConfig string

	// LogFormat controls slog output. "text" for human-readable dev output;
	// anything else (including "") uses structured JSON. Default: "json".
	LogFormat string

	// DeploymentProfile controls the egress posture for outbound fetches.
	// Default: "eu_hosted".
	DeploymentProfile string

	// GmailPubSubPushToken is the shared secret for the Gmail Pub/Sub webhook.
	GmailPubSubPushToken string

	// Blobstore is the object-storage backend (transcript audio + export bundles).
	// BlobstoreEnabled is false when the BLOBSTORE_* env is incomplete.
	Blobstore        blobstore.Config
	BlobstoreEnabled bool

	// STT* configure the optional speech-to-text transcription path (E04).
	STTProvider string
	STTProfile  string
	STTEndpoint string
	STTAPIKey   string

	// HubSpot OAuth app registration (B-E18.13). Empty ClientID disables the
	// integration's connect handler gracefully (404).
	HubSpotClientID     string
	HubSpotClientSecret string
	HubSpotRedirectURI  string

	// KeyVault is the token-vault sealing config (B-E18.13).
	KeyVault        keyvault.Config
	KeyVaultEnabled bool

	// OverlayBudgetRESTLimit/OverlayBudgetRESTCap configure the HubSpot
	// backfill's daily-REST overlaybudget.Meter entry.
	OverlayBudgetRESTLimit int64
	OverlayBudgetRESTCap   int64
}

// Load reads environment variables and applies defaults.
// Returns an error only for missing required fields.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		RedisURL:             getEnvDefault("REDIS_URL", "redis://localhost:6379/0"),
		Addr:                 getEnvDefault("ADDR", ":8080"),
		AIRoutingConfig:      os.Getenv("AI_ROUTING_CONFIG"),
		LogFormat:            getEnvDefault("LOG_FORMAT", "json"),
		DeploymentProfile:    getEnvDefault("DEPLOYMENT_PROFILE", "eu_hosted"),
		GmailPubSubPushToken: os.Getenv("GMAIL_PUBSUB_PUSH_TOKEN"),

		STTProvider: os.Getenv("MARGINCE_STT_PROVIDER"),
		STTProfile:  os.Getenv("MARGINCE_STT_PROFILE"),
		STTEndpoint: os.Getenv("MARGINCE_STT_ENDPOINT"),
		STTAPIKey:   os.Getenv("MARGINCE_STT_API_KEY"),

		OverlayBudgetRESTLimit: getEnvInt64Default("OVERLAY_BUDGET_HUBSPOT_REST_LIMIT", 40000),
		OverlayBudgetRESTCap:   getEnvInt64Default("OVERLAY_BUDGET_HUBSPOT_REST_CAP", 35000),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if bc, err := blobstore.LoadConfig(); err == nil {
		cfg.Blobstore = bc
		cfg.BlobstoreEnabled = true
	}

	cfg.HubSpotClientID = os.Getenv("HUBSPOT_CLIENT_ID")
	cfg.HubSpotClientSecret = os.Getenv("HUBSPOT_CLIENT_SECRET")
	cfg.HubSpotRedirectURI = os.Getenv("HUBSPOT_REDIRECT_URI")
	if kv, err := keyvault.LoadConfig(); err == nil {
		cfg.KeyVault = kv
		cfg.KeyVaultEnabled = true
	}

	return cfg, nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt64Default(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
