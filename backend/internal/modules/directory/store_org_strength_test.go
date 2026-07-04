//go:build integration

package crmcore

import (
	"context"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const orgAggTestWS = "00000000-0000-0000-0000-000000000021"

func TestOrgStore_List_AttachesAggregates(t *testing.T) {
	db := openTestDB(t)
	seedWorkspace(t, db, orgAggTestWS)
	setRLS(t, db, orgAggTestWS)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgAggTestWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	orgStore := NewOrgStore(db)
	personStore := NewPersonStore(db)

	orgWithContactsSeed := Organization{
		WorkspaceID: orgAggTestWS, DisplayName: "OrgWithContacts-" + uniq(),
		Source: "test", CapturedBy: "human:test",
	}
	orgWithContacts, err := orgStore.Create(ctx, orgWithContactsSeed)
	if err != nil {
		t.Fatalf("create orgWithContacts: %v", err)
	}

	emptyOrgSeed := Organization{
		WorkspaceID: orgAggTestWS, DisplayName: "EmptyOrg-" + uniq(),
		Source: "test", CapturedBy: "human:test",
	}
	emptyOrg, err := orgStore.Create(ctx, emptyOrgSeed)
	if err != nil {
		t.Fatalf("create emptyOrg: %v", err)
	}

	strongSeed := NewPerson("Strong-"+uniq(), p0)
	strongSeed.WorkspaceID = orgAggTestWS
	strongPerson, err := personStore.Create(ctx, strongSeed, nil)
	if err != nil {
		t.Fatalf("create strongPerson: %v", err)
	}

	weakSeed := NewPerson("Weak-"+uniq(), p0)
	weakSeed.WorkspaceID = orgAggTestWS
	_, err = personStore.Create(ctx, weakSeed, nil)
	if err != nil {
		t.Fatalf("create weakPerson: %v", err)
	}
	// employ both at orgWithContacts — we only need the ID from weak, reuse weakSeed after Create
	weakPerson, err := personStore.Create(ctx, func() Person {
		s := NewPerson("Weak2-"+uniq(), p0)
		s.WorkspaceID = orgAggTestWS
		return s
	}(), nil)
	if err != nil {
		t.Fatalf("create weakPerson2: %v", err)
	}

	for _, pid := range []string{strongPerson.ID, weakPerson.ID} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO relationship(workspace_id, kind, person_id, organization_id, role, source, captured_by)
			 VALUES ($1::uuid, 'employment', $2::uuid, $3::uuid, NULL, 'test', 'human:test')`,
			orgAggTestWS, pid, orgWithContacts.ID); err != nil {
			t.Fatalf("seed employment for %s: %v", pid, err)
		}
	}

	// Seed a recent email activity for strongPerson (within 90d window).
	actID := ids.New()
	occurred := fixedStrengthClock.AddDate(0, 0, -5)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO activity (id, workspace_id, kind, occurred_at, direction, source, captured_by, version)
		 VALUES ($1::uuid, $2::uuid, 'email', $3, 'inbound', 'test', 'human:test', 1)`,
		actID, orgAggTestWS, occurred); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	linkID := ids.New()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO activity_link (id, workspace_id, activity_id, entity_type, person_id)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, 'person', $4::uuid)`,
		linkID, orgAggTestWS, actID, strongPerson.ID); err != nil {
		t.Fatalf("seed activity_link: %v", err)
	}

	// Seed pipeline+stage for deal FKs.
	var pipeID, stageID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		orgAggTestWS, "pipe-"+uniq()).Scan(&pipeID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO stage (id, workspace_id, pipeline_id, name, position) VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`,
		orgAggTestWS, pipeID, "stage-"+uniq()).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, organization_id, status, source, captured_by, version)
		 VALUES (uuidv7(), $1, $2, $3::uuid, $4::uuid, $5::uuid, 'open', 'test', 'human:test', 1)`,
		orgAggTestWS, "OpenDeal-"+uniq(), pipeID, stageID, orgWithContacts.ID); err != nil {
		t.Fatalf("seed open deal: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, organization_id, status, closed_at, source, captured_by, version)
		 VALUES (uuidv7(), $1, $2, $3::uuid, $4::uuid, $5::uuid, 'won', NOW(), 'test', 'human:test', 1)`,
		orgAggTestWS, "WonDeal-"+uniq(), pipeID, stageID, orgWithContacts.ID); err != nil {
		t.Fatalf("seed won deal: %v", err)
	}

	orgs, _, err := orgStore.List(ctx, orgAggTestWS, "", 20, "", OrgListFilter{})
	if err != nil {
		t.Fatal(err)
	}

	var withContacts, empty *Organization
	for i := range orgs {
		if orgs[i].ID == orgWithContacts.ID {
			withContacts = &orgs[i]
		}
		if orgs[i].ID == emptyOrg.ID {
			empty = &orgs[i]
		}
	}

	if withContacts == nil {
		t.Fatal("org with contacts not found in list")
	}
	if withContacts.ContactCount != 2 {
		t.Fatalf("want contact_count=2, got %d", withContacts.ContactCount)
	}
	if withContacts.OpenDealCount != 1 {
		t.Fatalf("want open_deal_count=1 (won excluded), got %d", withContacts.OpenDealCount)
	}
	if withContacts.Strength == nil {
		t.Fatal("want org_strength non-nil, got nil")
	}
	if withContacts.Strength.TopPersonID != strongPerson.ID {
		t.Fatalf("want top_person_id=%s, got %s", strongPerson.ID, withContacts.Strength.TopPersonID)
	}

	if empty == nil {
		t.Fatal("empty org not found in list")
	}
	if empty.ContactCount != 0 || empty.OpenDealCount != 0 || empty.Strength != nil {
		t.Fatalf("want zero/nil for empty org, got contact_count=%d open_deal_count=%d org_strength=%+v",
			empty.ContactCount, empty.OpenDealCount, empty.Strength)
	}
}
