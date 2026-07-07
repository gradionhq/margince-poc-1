//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/gradionhq/margince/backend/internal/platform/events"
	"github.com/gradionhq/margince/backend/internal/platform/logger"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func testDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://margince:margince@localhost:5432/margince_test?sslmode=disable"
}

// testRedisClient honors REDIS_URL (including its /<db> index) so the parallel
// integration runner can pin this package to a private Redis logical db.
func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379"
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse REDIS_URL %q: %v", url, err)
	}
	return redis.NewClient(opts)
}

// TestTraceparentRecoverableAcrossRelay seeds an outbox row carrying a known
// traceparent, runs the REAL events.RelayBatch (outbox -> Redis), then uses
// events.ConsumerCtxFromStream to prove the downstream ctx carries the same
// trace id.
func TestTraceparentRecoverableAcrossRelay(t *testing.T) {
	db, err := sql.Open("postgres", testDBURL())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rdb := testRedisClient(t)
	defer rdb.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wsID := ids.New()
	if _, err := db.ExecContext(ctx, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "w"+wsID, "w"+wsID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `SELECT set_config('app.workspace_id',$1,false)`, wsID); err != nil {
		t.Fatal(err)
	}
	// Isolate from sibling tests' un-relayed outbox rows.
	if _, err := db.ExecContext(ctx, `UPDATE event_outbox SET published_at = now() WHERE published_at IS NULL`); err != nil {
		t.Fatal(err)
	}

	tid := "0af7651916cd43dd8448eb211c80319c"
	payload, _ := json.Marshal(map[string]any{"_traceparent": logger.FormatTraceparent(tid, "b7ad6b7169203331")})
	if _, err := db.ExecContext(ctx, `INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1::uuid,'audit.appended',$2::uuid,$3)`, wsID, ids.New(), payload); err != nil {
		t.Fatal(err)
	}

	if err := events.RelayBatch(ctx, db, rdb); err != nil {
		t.Fatalf("RelayBatch: %v", err)
	}
	// entityTypeFromTopic("audit.appended") == "audit"
	stream := events.StreamKey(wsID, "audit")
	res, err := rdb.XRange(ctx, stream, "-", "+").Result()
	if err != nil || len(res) == 0 {
		t.Fatalf("xrange %s: %v len=%d", stream, err, len(res))
	}
	cctx := events.ConsumerCtxFromStream(res[len(res)-1].Values)
	if logger.TraceID(cctx) != tid {
		t.Fatalf("downstream trace id = %q, want %q", logger.TraceID(cctx), tid)
	}
}
