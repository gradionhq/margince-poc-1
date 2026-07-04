//go:build integration

package crmcore

import (
	"context"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const strengthTestWS = "00000000-0000-0000-0000-000000000020"

func TestPersonStore_List_AttachesLastActivityAt(t *testing.T) {
	db := openTestDB(t)
	seedWorkspace(t, db, strengthTestWS)
	setRLS(t, db, strengthTestWS)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: strengthTestWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}
	store := NewPersonStore(db)

	// person WITH activity
	withSeed := NewPerson("Alice-Activity-"+uniq(), p0)
	withSeed.WorkspaceID = strengthTestWS
	personWith, err := store.Create(ctx, withSeed)
	if err != nil {
		t.Fatalf("create person with activity: %v", err)
	}

	// person WITHOUT activity
	withoutSeed := NewPerson("Bob-NoActivity-"+uniq(), p0)
	withoutSeed.WorkspaceID = strengthTestWS
	personWithout, err := store.Create(ctx, withoutSeed)
	if err != nil {
		t.Fatalf("create person without activity: %v", err)
	}

	t0 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	actID := ids.New()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO activity (id, workspace_id, kind, occurred_at, source, captured_by, version)
		 VALUES ($1::uuid, $2::uuid, 'email', $3, 'test', 'human:test', 1)`,
		actID, strengthTestWS, t0); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	linkID := ids.New()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO activity_link (id, workspace_id, activity_id, entity_type, person_id)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, 'person', $4::uuid)`,
		linkID, strengthTestWS, actID, personWith.ID); err != nil {
		t.Fatalf("seed activity_link: %v", err)
	}

	people, _, err := store.List(ctx, strengthTestWS, "", 20, "")
	if err != nil {
		t.Fatal(err)
	}

	var withAct, withoutAct *Person
	for i := range people {
		if people[i].ID == personWith.ID {
			withAct = &people[i]
		}
		if people[i].ID == personWithout.ID {
			withoutAct = &people[i]
		}
	}
	if withAct == nil {
		t.Fatal("person with activity not found in list")
	}
	if withAct.LastActivityAt == nil {
		t.Fatalf("want last_activity_at=%v, got nil", t0)
	}
	if !withAct.LastActivityAt.UTC().Equal(t0) {
		t.Fatalf("want last_activity_at=%v, got %v", t0, withAct.LastActivityAt.UTC())
	}
	if withoutAct == nil {
		t.Fatal("person without activity not found in list")
	}
	if withoutAct.LastActivityAt != nil {
		t.Fatalf("want nil last_activity_at for person with no activity, got %v", withoutAct.LastActivityAt)
	}
}
