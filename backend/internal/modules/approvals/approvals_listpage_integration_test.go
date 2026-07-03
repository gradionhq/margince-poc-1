//go:build integration

package crmapprovals_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

// TestListPage_CursorPagingAndFilters proves the real-DB data semantics the unit
// test cannot: cross-workspace exclusion, id-keyed cursor advance across pages,
// the kind filter, and a non-nil empty slice on an empty result.
func TestListPage_CursorPagingAndFilters(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	otherWsID := seedWorkspace(t, db)
	repo := crmapprovals.NewPageRepository()

	create := func(ws, action string) string {
		var id string
		withGUC(t, db, ws, func(tx *sql.Tx) {
			var err error
			id, err = repo.Create(context.Background(), tx, crmapprovals.Item{
				WorkspaceID: ws,
				ActionType:  action,
				Payload:     json.RawMessage(`{"kind":"person","id":"r1"}`),
				Status:      crmapprovals.StatusPending,
				RequestedBy: "agent:test",
			})
			if err != nil {
				t.Fatalf("create %s: %v", action, err)
			}
		})
		return id
	}

	// Stage 3 update_record + 2 send_email in wsID, and 1 in otherWsID.
	want := map[string]bool{}
	for i := 0; i < 3; i++ {
		want[create(wsID, "update_record")] = true
	}
	for i := 0; i < 2; i++ {
		want[create(wsID, "send_email")] = true
	}
	otherID := create(otherWsID, "update_record")

	// Page through wsID pending items with limit=2; collect every id via the cursor.
	got := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		var items []crmapprovals.Item
		var next string
		withGUC(t, db, wsID, func(tx *sql.Tx) {
			var err error
			items, next, err = repo.ListPage(context.Background(), tx, wsID, crmapprovals.StatusPending, "", cursor, 2)
			if err != nil {
				t.Fatalf("ListPage page %d: %v", pages, err)
			}
		})
		if items == nil {
			t.Fatal("ListPage must never return a nil slice")
		}
		for _, it := range items {
			if it.WorkspaceID != wsID {
				t.Fatalf("cross-workspace row leaked: %s in ws %s", it.ID, it.WorkspaceID)
			}
			got[it.ID] = true
		}
		pages++
		if next == "" {
			break
		}
		cursor = next
		if pages > 10 {
			t.Fatal("cursor failed to terminate")
		}
	}
	if len(got) != 5 {
		t.Fatalf("want 5 wsID items across pages, got %d", len(got))
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("paged result missing staged id %s", id)
		}
	}
	if got[otherID] {
		t.Fatalf("other-workspace id %s must be excluded", otherID)
	}
	if pages < 2 {
		t.Fatalf("limit=2 over 5 rows must span >1 page, got %d", pages)
	}

	// kind filter narrows to update_record only (3 of the 5).
	var filtered []crmapprovals.Item
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		var err error
		filtered, _, err = repo.ListPage(context.Background(), tx, wsID, crmapprovals.StatusPending, "update_record", "", 50)
		if err != nil {
			t.Fatalf("ListPage kind filter: %v", err)
		}
	})
	if len(filtered) != 3 {
		t.Fatalf("kind=update_record must narrow to 3, got %d", len(filtered))
	}
	for _, it := range filtered {
		if it.ActionType != "update_record" {
			t.Fatalf("kind filter leaked %s", it.ActionType)
		}
	}

	// Empty result (no rejected rows) → non-nil empty slice, empty cursor.
	var empty []crmapprovals.Item
	var emptyNext string
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		var err error
		empty, emptyNext, err = repo.ListPage(context.Background(), tx, wsID, crmapprovals.StatusRejected, "", "", 50)
		if err != nil {
			t.Fatalf("ListPage empty: %v", err)
		}
	})
	if empty == nil {
		t.Fatal("empty result must be a non-nil slice")
	}
	if len(empty) != 0 || emptyNext != "" {
		t.Fatalf("empty page want (0 items, \"\" cursor), got (%d, %q)", len(empty), emptyNext)
	}
}
