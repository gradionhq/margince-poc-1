//go:build integration

// store_org_strength_test.go — ported from modules/directory/store_org_strength_test.go
// (package crmcore → package adapters_test; person rows inserted via
// people/adapters.PersonStore since organizations/adapters only carries
// the minimal strength-query PersonStore, not full CRUD).
package adapters_test

import (
	"context"
	"testing"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgDomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	people "github.com/gradionhq/margince/backend/internal/modules/people"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const orgAggTestWS = "00000000-0000-0000-0000-000000000021"

func TestOrgStore_List_AttachesAggregates(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	pgtest.SeedWorkspace(t, db, orgAggTestWS)
	pgtest.SetRLS(t, db, orgAggTestWS)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgAggTestWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	orgStore := orgAdapters.NewOrgStore(db)
	personStore := people.NewPersonStore(db)

	orgWithContactsSeed := orgDomain.Organization{
		WorkspaceID: orgAggTestWS, DisplayName: "OrgWithContacts-" + pgtest.Uniq(),
		Source: "test", CapturedBy: "human:test",
	}
	orgWithContacts, err := orgStore.Create(ctx, orgWithContactsSeed, nil)
	if err != nil {
		t.Fatalf("create orgWithContacts: %v", err)
	}

	emptyOrgSeed := orgDomain.Organization{
		WorkspaceID: orgAggTestWS, DisplayName: "EmptyOrg-" + pgtest.Uniq(),
		Source: "test", CapturedBy: "human:test",
	}
	emptyOrg, err := orgStore.Create(ctx, emptyOrgSeed, nil)
	if err != nil {
		t.Fatalf("create emptyOrg: %v", err)
	}

	strongSeed := people.NewPerson("Strong-"+pgtest.Uniq(), p0)
	strongSeed.WorkspaceID = orgAggTestWS
	strongPerson, err := personStore.Create(ctx, strongSeed, nil)
	if err != nil {
		t.Fatalf("create strongPerson: %v", err)
	}

	weakSeed := people.NewPerson("Weak-"+pgtest.Uniq(), p0)
	weakSeed.WorkspaceID = orgAggTestWS
	_, err = personStore.Create(ctx, weakSeed, nil)
	if err != nil {
		t.Fatalf("create weakPerson: %v", err)
	}
	// employ both at orgWithContacts — we only need the ID from weak2, create a second weak person
	weakPerson, err := personStore.Create(ctx, func() people.Person {
		s := people.NewPerson("Weak2-"+pgtest.Uniq(), p0)
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
		orgAggTestWS, "pipe-"+pgtest.Uniq()).Scan(&pipeID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO stage (id, workspace_id, pipeline_id, name, position) VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`,
		orgAggTestWS, pipeID, "stage-"+pgtest.Uniq()).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, organization_id, status, source, captured_by, version)
		 VALUES (uuidv7(), $1, $2, $3::uuid, $4::uuid, $5::uuid, 'open', 'test', 'human:test', 1)`,
		orgAggTestWS, "OpenDeal-"+pgtest.Uniq(), pipeID, stageID, orgWithContacts.ID); err != nil {
		t.Fatalf("seed open deal: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, organization_id, status, closed_at, source, captured_by, version)
		 VALUES (uuidv7(), $1, $2, $3::uuid, $4::uuid, $5::uuid, 'won', NOW(), 'test', 'human:test', 1)`,
		orgAggTestWS, "WonDeal-"+pgtest.Uniq(), pipeID, stageID, orgWithContacts.ID); err != nil {
		t.Fatalf("seed won deal: %v", err)
	}

	orgs, _, err := orgStore.List(ctx, orgAggTestWS, "", 20, "", orgDomain.OrgListFilter{})
	if err != nil {
		t.Fatal(err)
	}

	var withContacts, empty *orgDomain.Organization
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
