//go:build integration

package adapters

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
)

// seedActivityVolume inserts n activity rows spread across kinds and subjects
// so the p95 assertions exercise a realistic working set.
func seedActivityVolume(t *testing.T, db *sql.DB, wsID string, n int) {
	t.Helper()
	kinds := []string{"email", "call", "meeting", "note", "task", "whatsapp", "telegram"}
	now := time.Now().UTC()
	for i := 0; i < n; i++ {
		subject := fmt.Sprintf("activity %d proposal review", i)
		if i%3 != 0 {
			subject = fmt.Sprintf("activity %d routine update", i)
		}
		if _, err := db.Exec(`INSERT INTO activity (id, workspace_id, kind, subject, is_done, occurred_at, source, captured_by)
			VALUES (uuidv7(), $1, $2, $3, false, $4, 'test', 'human:test')`,
			wsID, kinds[i%len(kinds)], subject, now.Add(-time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("seed activity volume row %d: %v", i, err)
		}
	}
}

func p95(durations []time.Duration) time.Duration {
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	idx := int(float64(len(durations))*0.95) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(durations) {
		idx = len(durations) - 1
	}
	return durations[idx]
}

func TestActivityStore_ListFiltered_P95FilteredRead_UnderBudget(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "perf-filtered")
	seedActivityVolume(t, db, wsID, 1500)
	s := NewActivityStore(db)

	const iterations = 20
	durations := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		if _, _, err := s.ListFiltered(context.Background(), wsID, "", 50, domain.ActivityListFilter{Kind: "task"}); err != nil {
			t.Fatalf("ListFiltered: %v", err)
		}
		durations = append(durations, time.Since(start))
	}
	if got := p95(durations); got > 150*time.Millisecond {
		t.Fatalf("p95 filtered-list latency = %v, want < 150ms (PERF-2), all durations: %v", got, durations)
	}
}

func TestActivityStore_ListFiltered_P95FullTextSearch_UnderBudget(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "perf-q")
	seedActivityVolume(t, db, wsID, 1500)
	s := NewActivityStore(db)

	const iterations = 20
	durations := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		if _, _, err := s.ListFiltered(context.Background(), wsID, "", 50, domain.ActivityListFilter{Q: "proposal"}); err != nil {
			t.Fatalf("ListFiltered q=proposal: %v", err)
		}
		durations = append(durations, time.Since(start))
	}
	if got := p95(durations); got > 200*time.Millisecond {
		t.Fatalf("p95 full-text-search latency = %v, want < 200ms (PERF-3), all durations: %v", got, durations)
	}
}
