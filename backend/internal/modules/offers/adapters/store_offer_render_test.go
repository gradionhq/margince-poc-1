//go:build integration

package adapters_test

import (
	"context"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

func TestOfferStore_PrepareRender_DraftNoBuyerOrg_OmitsBuyerBlock(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	offerStore := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-PR-"+wsID[:8], "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := offerStore.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	ing, err := offerStore.PrepareRender(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("prepare render: %v", err)
	}
	if ing.BuyerBlock != nil {
		t.Fatalf("expected a nil buyer block for a buyer_org_id-less draft, got %+v", ing.BuyerBlock)
	}
	if ing.Locale != "de-DE" {
		t.Fatalf("expected default locale de-DE, got %q", ing.Locale)
	}
	if ing.IssuerName == "" {
		t.Fatal("expected a non-empty live issuer name")
	}
}

func TestOfferStore_PrepareRender_Sent_UsesFrozenBuyerSnapshot(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	offerStore := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-PR2-"+wsID[:8], "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := offerStore.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	sent, err := offerStore.Send(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	ing, err := offerStore.PrepareRender(context.Background(), sent.ID, wsID)
	if err != nil {
		t.Fatalf("prepare render: %v", err)
	}
	if ing.BuyerBlock == nil {
		t.Fatal("expected the frozen buyer_snapshot to be used post-send")
	}
}

func TestOfferStore_SetPdfAssetRef_PersistsAndAudits(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	offerStore := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-PR3-"+wsID[:8], "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := offerStore.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := offerStore.SetPdfAssetRef(context.Background(), created.ID, wsID, "offers/"+wsID+"/"+created.ID+"/1.pdf")
	if err != nil {
		t.Fatalf("set pdf asset ref: %v", err)
	}
	if updated.PdfAssetRef == nil || *updated.PdfAssetRef == "" {
		t.Fatalf("expected pdf_asset_ref populated, got %+v", updated.PdfAssetRef)
	}

	var auditCount int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE action='update' AND entity_id=$1::uuid`, created.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected exactly one render audit_log entry, got %d", auditCount)
	}
}
