package main

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/v9"
)

// newRedisClient builds a Redis client from cfg.RedisURL.
// Returns (nil, false) when the URL is empty — callers treat this as "relay disabled".
func newRedisClient(cfg Config) (*redis.Client, bool) {
	if cfg.RedisURL == "" {
		slog.Warn("outbox relay disabled: RedisURL not set")
		return nil, false
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Warn("outbox relay disabled: parse RedisURL", "err", err)
		return nil, false
	}
	return redis.NewClient(opts), true
}

// streamKey returns the Redis Streams key for the given workspace and entity type.
// Convention: margince:events:{workspace_id}:{entity_type}
// e.g. margince:events:abc123:person
func streamKey(workspaceID, entityType string) string {
	return fmt.Sprintf("margince:events:%s:%s", workspaceID, entityType)
}

// entityTypeFromTopic extracts the entity type from a "<entity>.<verb>" topic string.
// "person.created" → "person". Returns "unknown" for malformed topics.
func entityTypeFromTopic(topic string) string {
	parts := strings.SplitN(topic, ".", 2)
	if len(parts) < 2 || parts[0] == "" {
		return "unknown"
	}
	return parts[0]
}
