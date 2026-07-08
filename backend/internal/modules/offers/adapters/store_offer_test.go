//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/lib/pq" // registers the postgres driver for database/sql

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// seedOfferWorkspace seeds a workspace + pipeline + stage + deal (offer's
// only hard FK), mirroring activities/adapters/store_activity_test.go's
// seedActivityStoreFixtures fixture chain, and sets the RLS GUC.
func seedOfferWorkspace(t *testing.T, db *sql.DB) (wsID, dealID string) {
	t.Helper()
	tag := "op-t05-" + time.Now().Format("20060102150405.000000000")
	if err := db.QueryRow(`INSERT INTO workspace (id, name, slug, base_currency)
		VALUES (uuidv7(), $1, $2, 'EUR') RETURNING id::text`,
		"t-"+tag, "t-"+tag).Scan(&wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, wsID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	var pipelineID, stageID string
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		wsID, "P-"+tag).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position) VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`,
		wsID, pipelineID, "S-"+tag).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, source, captured_by)
		VALUES (uuidv7(), $1, $2, $3, $4, 'test', 'human:test') RETURNING id`,
		wsID, "D-"+tag, pipelineID, stageID).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	return wsID, dealID
}

func provTestOffer() prov.Provenance {
	return prov.Provenance{Source: "test", CapturedBy: "human:test"}
}

func TestOfferStore_CreateGetListUpdate_RoundTrip(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-"+ids.New(), "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := s.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != domain.OfferStatusDraft || created.Revision != 1 {
		t.Fatalf("expected draft/revision=1, got status=%s revision=%d", created.Status, created.Revision)
	}
	if created.NetMinor != 0 || created.TaxMinor != 0 || created.GrossMinor != 0 {
		t.Fatalf("expected zero totals on a lineless offer, got net=%d tax=%d gross=%d", created.NetMinor, created.TaxMinor, created.GrossMinor)
	}

	got, err := s.Get(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.OfferNumber != created.OfferNumber {
		t.Fatalf("get: offer_number mismatch")
	}

	items, next, err := s.List(context.Background(), wsID, dealID, "", 20, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("expected the one created offer, got %+v (next=%q)", items, next)
	}

	updated, err := s.Update(context.Background(), created.ID, wsID, map[string]any{"intro_text": "Hello"}, created.Version)
	if err != nil {
		t.Fatalf("update while draft: %v", err)
	}
	if updated.IntroText == nil || *updated.IntroText != "Hello" {
		t.Fatalf("expected intro_text updated, got %+v", updated.IntroText)
	}
	if updated.Version != created.Version+1 {
		t.Fatalf("expected version bump to %d, got %d", created.Version+1, updated.Version)
	}
}

func TestOfferStore_Create_DuplicateOfferNumberRevision_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)

	num := "ANG-" + ids.New()
	base := domain.NewOffer(dealID, num, "EUR", provTestOffer())
	base.WorkspaceID = wsID
	if _, err := s.Create(context.Background(), base); err != nil {
		t.Fatalf("first create: %v", err)
	}
	dup := domain.NewOffer(dealID, num, "EUR", provTestOffer())
	dup.WorkspaceID = wsID
	_, err := s.Create(context.Background(), dup)
	if !errors.Is(err, adapters.ErrDuplicateOfferNumber) {
		t.Fatalf("expected ErrDuplicateOfferNumber, got %v", err)
	}
}

func TestOfferStore_Create_UnknownDeal_NotFound(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, _ := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)

	o := domain.NewOffer(ids.New(), "ANG-"+ids.New(), "EUR", provTestOffer())
	o.WorkspaceID = wsID
	_, err := s.Create(context.Background(), o)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a deal_id that doesn't exist, got %v", err)
	}
}

func TestOfferStore_Create_MissingProvenance_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)

	o := domain.Offer{WorkspaceID: wsID, DealID: dealID, OfferNumber: "X", Currency: "EUR", Status: "draft", Revision: 1, Version: 1}
	_, err := s.Create(context.Background(), o)
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Fatalf("expected ErrNullProvenance, got %v", err)
	}
}

func TestOfferStore_Update_NonDraft_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-"+ids.New(), "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := s.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`UPDATE offer SET status='sent' WHERE id=$1::uuid`, created.ID); err != nil {
		t.Fatalf("force status=sent: %v", err)
	}
	_, err = s.Update(context.Background(), created.ID, wsID, map[string]any{"intro_text": "nope"}, 0)
	if !errors.Is(err, adapters.ErrOfferNotDraft) {
		t.Fatalf("expected ErrOfferNotDraft, got %v", err)
	}
}

func TestOfferStore_Update_VersionSkew_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-"+ids.New(), "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := s.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = s.Update(context.Background(), created.ID, wsID, map[string]any{"intro_text": "x"}, created.Version+99)
	if !errors.Is(err, errs.ErrVersionSkew) {
		t.Fatalf("expected ErrVersionSkew, got %v", err)
	}
}

// TestOfferStore_Update_UuidFKColumns_NoTypeMismatch guards the ::uuid casts
// on buyer_org_id/template_id's CASE branches (PLAN-review finding: a bare
// CASE unifying a text param against a uuid column errors "types uuid and
// text cannot be matched" — see store_offer.go's Update doc comment). Seeds
// a real organization + offer_template row so the FK constraints are
// satisfied too, not just the type-check.
func TestOfferStore_Update_UuidFKColumns_NoTypeMismatch(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)

	var orgID string
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, source, captured_by)
		VALUES (uuidv7(), $1, 'Acme', 'test', 'human:test') RETURNING id`, wsID).Scan(&orgID); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	var templateID string
	if err := db.QueryRow(`INSERT INTO offer_template (id, workspace_id, name, layout, version)
		VALUES (uuidv7(), $1, 'Default DE', '{}'::jsonb, 1) RETURNING id`, wsID).Scan(&templateID); err != nil {
		t.Fatalf("seed offer_template: %v", err)
	}

	o := domain.NewOffer(dealID, "ANG-"+ids.New(), "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := s.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := s.Update(context.Background(), created.ID, wsID,
		map[string]any{"buyer_org_id": orgID, "template_id": templateID}, 0)
	if err != nil {
		t.Fatalf("update buyer_org_id/template_id: %v", err)
	}
	if updated.BuyerOrgID == nil || *updated.BuyerOrgID != orgID {
		t.Fatalf("expected buyer_org_id=%s, got %+v", orgID, updated.BuyerOrgID)
	}
	if updated.TemplateID == nil || *updated.TemplateID != templateID {
		t.Fatalf("expected template_id=%s, got %+v", templateID, updated.TemplateID)
	}
}

func TestOfferStore_Regenerate_SentOffer_ClonesAndSupersedes(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)
	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))

	num := "ANG-" + ids.New()
	original := domain.NewOffer(dealID, num, "EUR", provTestOffer())
	original.WorkspaceID = wsID
	created, err := s.Create(context.Background(), original)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	li := domain.OfferLineItem{
		WorkspaceID: wsID, OfferID: created.ID, Position: 1,
		Description: "Consulting Day", Unit: "day", Quantity: 2, UnitPriceMinor: 50000,
		Source: "test", CapturedBy: "human:test",
	}
	if _, err := lineStore.Create(context.Background(), li, nil); err != nil {
		t.Fatalf("create line: %v", err)
	}
	if _, err := db.Exec(`UPDATE offer SET status=$2 WHERE id=$1::uuid`, created.ID, domain.OfferStatusSent); err != nil {
		t.Fatalf("force status=sent: %v", err)
	}

	regenerated, err := s.Regenerate(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if regenerated.ID == created.ID {
		t.Fatal("expected a new offer id")
	}
	if regenerated.Status != domain.OfferStatusDraft {
		t.Fatalf("expected regenerated offer to be draft, got %s", regenerated.Status)
	}
	if regenerated.Revision != created.Revision+1 {
		t.Fatalf("expected revision %d, got %d", created.Revision+1, regenerated.Revision)
	}
	if regenerated.OfferNumber != created.OfferNumber {
		t.Fatalf("expected offer_number to be cloned, got %s want %s", regenerated.OfferNumber, created.OfferNumber)
	}
	if regenerated.DealID != created.DealID {
		t.Fatalf("expected deal_id to be cloned, got %s want %s", regenerated.DealID, created.DealID)
	}
	if regenerated.Source != created.Source || regenerated.CapturedBy != created.CapturedBy {
		t.Fatalf("expected provenance to be cloned, got source=%s captured_by=%s", regenerated.Source, regenerated.CapturedBy)
	}

	originalRow, err := s.Get(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("get original: %v", err)
	}
	if originalRow.Status != domain.OfferStatusSuperseded {
		t.Fatalf("expected original to be superseded, got %s", originalRow.Status)
	}

	items, err := lineStore.List(context.Background(), regenerated.ID, wsID)
	if err != nil {
		t.Fatalf("list regenerated lines: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one cloned line item, got %d", len(items))
	}
	if items[0].Description != li.Description || items[0].UnitPriceMinor != li.UnitPriceMinor || items[0].Quantity != li.Quantity {
		t.Fatalf("expected cloned line item fields to match source, got %+v", items[0])
	}
}
