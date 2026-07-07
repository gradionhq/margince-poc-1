// Package events holds the cross-cutting event infrastructure: outbox relay,
// River background-job client setup, and the Tier-0 workflow seam types
// (EventEnvelope, Handler) previously in internal/shared/ports/workflow.
// Moved here from cmd/api by WS-E-d (Task 8, AC-E4).
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/gradionhq/margince/backend/internal/platform/logger"
)

// NewRedisClient builds a Redis client from the given URL.
// Returns (nil, false) when the URL is empty or unparseable — callers treat
// this as "relay disabled".
func NewRedisClient(redisURL string) (*redis.Client, bool) {
	if redisURL == "" {
		return nil, false
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, false
	}
	return redis.NewClient(opts), true
}

// StreamKey returns the Redis Streams key for a workspace + entity type pair.
// Convention: margince:events:{workspace_id}:{entity_type}
func StreamKey(workspaceID, entityType string) string {
	return fmt.Sprintf("margince:events:%s:%s", workspaceID, entityType)
}

func entityTypeFromTopic(topic string) string {
	parts := strings.SplitN(topic, ".", 2)
	if len(parts) < 2 || parts[0] == "" {
		return "unknown"
	}
	return parts[0]
}

// StartOutboxRelay launches a background goroutine that polls event_outbox for
// unpublished rows, publishes them to Redis Streams via XADD, then marks them
// published. It exits when ctx is cancelled.
//
// appDB must be a superuser connection (DATABASE_URL as-is) so it can read
// across all workspace_ids without RLS filtering.
func StartOutboxRelay(ctx context.Context, appDB *sql.DB, rdb *redis.Client) {
	go func() {
		log.Println("outbox relay started")
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("outbox relay stopped")
				return
			case <-ticker.C:
				if err := RelayBatch(ctx, appDB, rdb); err != nil {
					log.Printf("outbox relay: batch error: %v", err)
				}
			}
		}
	}()
}

// RelayBatch reads up to 100 unpublished rows from event_outbox, publishes each
// to Redis Streams, then marks the row published. Exported for integration tests.
func RelayBatch(ctx context.Context, db *sql.DB, rdb *redis.Client) error {
	const q = `
		SELECT id, workspace_id, topic, entity_id, payload
		FROM event_outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT 100`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			id          string
			workspaceID string
			topic       string
			entityID    string
			payload     []byte
		)
		if err := rows.Scan(&id, &workspaceID, &topic, &entityID, &payload); err != nil {
			return err
		}

		entityType := entityTypeFromTopic(topic)
		key := StreamKey(workspaceID, entityType)

		tp := traceparentFromPayload(payload)

		if err := rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: key,
			Values: map[string]any{
				"event_id":    id,
				"topic":       topic,
				"payload":     string(payload),
				"traceparent": tp,
			},
		}).Err(); err != nil {
			return err
		}

		const mark = `UPDATE event_outbox SET published_at = now() WHERE id = $1`
		if _, err := db.ExecContext(ctx, mark, id); err != nil {
			return err
		}
	}
	return rows.Err()
}

func traceparentFromPayload(payload []byte) string {
	if tp, ok := carriedTraceparent(payload); ok {
		return tp
	}
	return logger.FormatTraceparent(logger.NewTraceID(), logger.NewSpanID())
}

func carriedTraceparent(payload []byte) (string, bool) {
	if len(payload) == 0 {
		return "", false
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return "", false
	}
	tp, ok := m["_traceparent"].(string)
	if !ok || tp == "" {
		return "", false
	}
	if _, _, ok := logger.ParseTraceparent(tp); !ok {
		return "", false
	}
	return tp, true
}

// ConsumerCtxFromStream reconstructs a context carrying the trace from a Redis
// stream entry's values. Falls back to a fresh trace if the field is absent or
// unparseable. Exported for integration tests.
func ConsumerCtxFromStream(values map[string]any) context.Context {
	ctx := context.Background()
	if tp, ok := values["traceparent"].(string); ok {
		if tid, sid, ok := logger.ParseTraceparent(tp); ok {
			return logger.WithTrace(ctx, tid, sid)
		}
	}
	return logger.WithTrace(ctx, logger.NewTraceID(), logger.NewSpanID())
}
