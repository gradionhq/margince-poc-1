//go:build integration

package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const personCompositeWS = "00000000-0000-0000-0000-000000000041"

func TestPersonHandler_Get_Composite360(t *testing.T) {
	db := openTestDB(t)
	seedWorkspace(t, db, personCompositeWS)
	setRLS(t, db, personCompositeWS)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: personCompositeWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	personStore := directory.NewPersonStore(db)
	relStore := directory.NewRelationshipStore(db)
	dealStore := directory.NewDealStore(db)
	activityStore := directory.NewActivityStore(db)
	h := NewPersonHandler(personStore, relStore, dealStore, activityStore)

	subject, err := personStore.Create(ctx, directory.Person{WorkspaceID: personCompositeWS, FullName: "Composite Subject", Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed subject: %v", err)
	}
	org, err := directory.NewOrgStore(db).Create(ctx, directory.Organization{WorkspaceID: personCompositeWS, DisplayName: "Composite Org", Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO relationship (workspace_id, kind, person_id, organization_id, is_primary, source, captured_by)
		VALUES ($1,'employment',$2,$3,false,$4,$5)`,
		personCompositeWS, subject.ID, org.ID, p0.Source, p0.CapturedBy); err != nil {
		t.Fatalf("seed relationship: %v", err)
	}
	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: personCompositeWS, Name: "Composite Pipeline"})
	if err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	sstore := deals.NewStageStore(db)
	st, err := sstore.Create(ctx, deals.Stage{WorkspaceID: personCompositeWS, PipelineID: pl.ID, Name: "Open", Position: 1, Semantic: "open", WinProbability: 50})
	if err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	d := directory.NewDeal("Composite Deal", pl.ID, st.ID, p0)
	d.WorkspaceID = personCompositeWS
	created, err := dealStore.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO relationship (workspace_id, kind, person_id, deal_id, role, is_primary, source, captured_by)
		VALUES ($1,'deal_stakeholder',$2,$3,$4,false,$5,$6)`,
		personCompositeWS, subject.ID, created.ID, stringPtr("champion"), p0.Source, p0.CapturedBy); err != nil {
		t.Fatalf("seed deal stakeholder: %v", err)
	}
	act, err := activityStore.Create(ctx, directory.Activity{WorkspaceID: personCompositeWS, Kind: "call", OccurredAt: time.Now(), Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO activity_link (workspace_id, activity_id, entity_type, person_id) VALUES ($1,$2,'person',$3)`, personCompositeWS, act.ID, subject.ID); err != nil {
		t.Fatalf("link activity: %v", err)
	}

	other, err := personStore.Create(ctx, directory.Person{WorkspaceID: personCompositeWS, FullName: "Unrelated Person", Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed other: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO relationship (workspace_id, kind, person_id, organization_id, is_primary, source, captured_by)
		VALUES ($1,'employment',$2,$3,false,$4,$5)`,
		personCompositeWS, other.ID, org.ID, p0.Source, p0.CapturedBy); err != nil {
		t.Fatalf("seed unrelated relationship: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/people/"+subject.ID, nil)
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{TenantID: personCompositeWS, UserID: "human:test"}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /people/{id}: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var got directory.Person
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Relationships) != 2 {
		t.Fatalf("relationships = %+v; want exactly two linked rows", got.Relationships)
	}
	var sawEmployment, sawStakeholder bool
	for _, rel := range got.Relationships {
		switch rel.Kind {
		case "employment":
			if rel.OrganizationID != nil && *rel.OrganizationID == org.ID {
				sawEmployment = true
			}
		case "deal_stakeholder":
			if rel.DealID != nil && *rel.DealID == created.ID {
				sawStakeholder = true
			}
		}
	}
	if !sawEmployment || !sawStakeholder {
		t.Fatalf("relationships = %+v; want employment->org %s and deal_stakeholder->deal %s", got.Relationships, org.ID, created.ID)
	}
	if len(got.Deals) != 1 || got.Deals[0].ID != created.ID {
		t.Fatalf("deals = %+v; want exactly one, id %s", got.Deals, created.ID)
	}
	if len(got.Activities) != 1 || got.Activities[0].ID != act.ID {
		t.Fatalf("activities = %+v; want exactly one, id %s", got.Activities, act.ID)
	}
}

func stringPtr(s string) *string { return &s }

func TestPersonHandler_Get_ArchivedStillFetchableWithComposite(t *testing.T) {
	db := openTestDB(t)
	seedWorkspace(t, db, personCompositeWS)
	setRLS(t, db, personCompositeWS)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: personCompositeWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	personStore := directory.NewPersonStore(db)
	h := NewPersonHandler(personStore, directory.NewRelationshipStore(db), directory.NewDealStore(db), directory.NewActivityStore(db))

	p, err := personStore.Create(ctx, directory.Person{WorkspaceID: personCompositeWS, FullName: "Archive Me", Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := personStore.Archive(ctx, p.ID, personCompositeWS); err != nil {
		t.Fatalf("archive: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/people/"+p.ID, nil)
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{TenantID: personCompositeWS, UserID: "human:test"}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("archived person GET: want 200 (still fetchable by id), got %d: %s", w.Code, w.Body.String())
	}
}
