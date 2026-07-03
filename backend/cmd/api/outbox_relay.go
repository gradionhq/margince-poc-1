package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	obs "github.com/gradionhq/margince/backend/internal/shared/kernel/obs"
)

// startOutboxRelay launches a background goroutine that polls event_outbox for
// unpublished rows, publishes them to Redis Streams via XADD, then marks them
// published. It exits when ctx is cancelled.
//
// appDB must be a superuser connection (DATABASE_URL as-is) so it can read
// across all workspace_ids without RLS filtering.
func startOutboxRelay(ctx context.Context, appDB *sql.DB, rdb *redis.Client) {
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
				if err := relayBatch(ctx, appDB, rdb); err != nil {
					log.Printf("outbox relay: batch error: %v", err)
				}
			}
		}
	}()
}

// relayBatch reads up to 100 unpublished rows from event_outbox, publishes each
// to Redis Streams, then marks the row published.
func relayBatch(ctx context.Context, db *sql.DB, rdb *redis.Client) error {
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
		key := streamKey(workspaceID, entityType)

		// Carry traceparent from the outbox payload's _traceparent field, or mint one.
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

// traceparentFromPayload extracts _traceparent from the outbox payload JSON, or
// mints a fresh one so the boundary span is always recoverable downstream.
func traceparentFromPayload(payload []byte) string {
	if tp, ok := carriedTraceparent(payload); ok {
		return tp
	}
	return obs.FormatTraceparent(obs.NewTraceID(), obs.NewSpanID())
}

// carriedTraceparent returns the valid _traceparent embedded in payload, if any.
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
	if _, _, ok := obs.ParseTraceparent(tp); !ok {
		return "", false
	}
	return tp, true
}

// consumerCtxFromStream reconstructs a context carrying the trace from a Redis
// stream entry's values. Falls back to a fresh trace if the field is absent or
// unparseable.
func consumerCtxFromStream(values map[string]any) context.Context {
	ctx := context.Background()
	if tp, ok := values["traceparent"].(string); ok {
		if tid, sid, ok := obs.ParseTraceparent(tp); ok {
			return obs.WithTrace(ctx, tid, sid)
		}
	}
	return obs.WithTrace(ctx, obs.NewTraceID(), obs.NewSpanID())
}
