package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gradionhq/margince/backend/internal/platform/blobstore"
	"github.com/gradionhq/margince/backend/internal/platform/keyvault"
)

// Config holds all runtime configuration for the server.
//
// This is the server's single composition root for the environment: every env
// var the cmd/server binary reads is resolved here, once, at startup. No other
// file in cmd/server calls os.Getenv (enforced by env_guard_test.go). Typed
// sub-config that lives in its own package (e.g. blobstore.Config) is loaded via
// that package's loader from here, so the "resolve once" invariant still holds.
//
// The separate cmd/crm-mcp binary has its own root and reads MCP_* itself — it is
// intentionally out of scope for this struct.
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

	// DeploymentProfile controls the egress posture for outbound fetches
	// (e.g. coldstart web reads). Default: "eu_hosted".
	//
	// NOTE: this is *not* the AI-routing profile. The AI router's profile comes
	// solely from the `profile:` field of ai-routing.yaml; this var never overrides it.
	DeploymentProfile string

	// GmailPubSubPushToken is the shared secret for the Gmail Pub/Sub webhook.
	GmailPubSubPushToken string

	// Blobstore is the object-storage backend (transcript audio + export bundles).
	// BlobstoreEnabled is false when the BLOBSTORE_* env is incomplete, in which
	// case the transcript-ingest worker and the export-to-blob path are disabled
	// gracefully (the server still boots and core CRUD is unaffected).
	Blobstore        blobstore.Config
	BlobstoreEnabled bool

	// STT* configure the optional speech-to-text transcription path (E04).
	// STTProvider empty disables transcription entirely; the capture pipeline still
	// runs for already-transcribed text. Profile/Endpoint/APIKey feed the chosen
	// provider (e.g. a local Whisper/Ollama endpoint under the sovereign profile).
	STTProvider string
	STTProfile  string
	STTEndpoint string
	STTAPIKey   string

	// HubSpotClientID/HubSpotClientSecret/HubSpotRedirectURI are the OAuth app
	// registration for the HubSpot connector (B-E18.13). Empty ClientID
	// disables the integration's connect handler gracefully (404).
	HubSpotClientID     string
	HubSpotClientSecret string
	HubSpotRedirectURI  string

	// KeyVault is the token-vault sealing config (B-E18.13). Required for the
	// HubSpot connector to be enabled — the server fails startup if
	// KEYVAULT_MASTER_KEY is malformed but present; if entirely unset and
	// HubSpotClientID is also unset, the integration is simply disabled.
	KeyVault        keyvault.Config
	KeyVaultEnabled bool

	// OverlayBudgetRESTLimit/OverlayBudgetRESTCap configure the HubSpot
	// backfill's daily-REST overlaybudget.Meter entry.
	OverlayBudgetRESTLimit int64
	OverlayBudgetRESTCap   int64
}

// loadConfig reads environment variables and applies defaults.
// Returns an error only for missing required fields.
func loadConfig() (Config, error) {
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

	// Blobstore is optional: an incomplete BLOBSTORE_* set disables the feature
	// rather than aborting boot (matches the prior river.go behavior).
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
