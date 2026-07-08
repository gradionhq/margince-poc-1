//go:build integration

package adapters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

func TestOfferStore_Send_SameCurrency_NoFXLookupNeeded(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	offerStore := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-SND-"+wsID[:8], "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := offerStore.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	sent, err := offerStore.Send(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if sent.Status != domain.OfferStatusSent {
		t.Fatalf("expected status=sent, got %s", sent.Status)
	}
	if sent.FxRateToBase == nil || *sent.FxRateToBase != "1.0000000000" {
		t.Fatalf("expected identity fx_rate_to_base=1.0000000000, got %+v", sent.FxRateToBase)
	}
	if sent.IssuerSnapshot == nil || sent.IssuerSnapshot["name"] == nil {
		t.Fatalf("expected issuer_snapshot populated, got %+v", sent.IssuerSnapshot)
	}
	if sent.BuyerSnapshot == nil {
		t.Fatalf("expected buyer_snapshot={} (never nil) for a buyer_org_id-less offer, got nil")
	}

	var eventCount int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='offer.sent' AND entity_id=$1::uuid`, created.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected exactly one offer.sent event, got %d", eventCount)
	}
}

func TestOfferStore_Send_CrossCurrency_NoStoredRate_FXUnavailable(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	offerStore := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-SND2-"+ids.New(), "USD", provTestOffer())
	o.WorkspaceID = wsID
	created, err := offerStore.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = offerStore.Send(context.Background(), created.ID, wsID)
	var fxErr *deals.FXRateUnavailableError
	if !errors.As(err, &fxErr) {
		t.Fatalf("expected *deals.FXRateUnavailableError, got %v", err)
	}
}

func TestOfferStore_Send_CrossCurrency_WithStoredRate_Freezes(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	if _, err := db.Exec(`INSERT INTO fx_rate (workspace_id, from_currency, to_currency, rate, rate_date)
		VALUES ($1,'USD','EUR',0.92,current_date)`, wsID); err != nil {
		t.Fatalf("seed fx_rate: %v", err)
	}
	offerStore := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-SND3-"+ids.New(), "USD", provTestOffer())
	o.WorkspaceID = wsID
	created, err := offerStore.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	sent, err := offerStore.Send(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if sent.FxRateToBase == nil || *sent.FxRateToBase == "1.0000000000" {
		t.Fatalf("expected a non-identity frozen rate, got %+v", sent.FxRateToBase)
	}
}

func TestOfferStore_Send_NotDraft_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	offerStore := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-SND4-"+ids.New(), "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := offerStore.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := offerStore.Send(context.Background(), created.ID, wsID); err != nil {
		t.Fatalf("first send: %v", err)
	}
	_, err = offerStore.Send(context.Background(), created.ID, wsID)
	if !errors.Is(err, adapters.ErrOfferNotDraft) {
		t.Fatalf("expected ErrOfferNotDraft on a second send, got %v", err)
	}
}

func TestOfferStore_Send_BuyerOrgSet_SnapshotsDisplayNameAndAddress(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	var orgID string
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, address, source, captured_by)
		VALUES (uuidv7(), $1, 'Acme GmbH', '{"city":"Berlin"}'::jsonb, 'test', 'human:test') RETURNING id`,
		wsID).Scan(&orgID); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	offerStore := adapters.NewOfferStore(db)

	o := domain.NewOffer(dealID, "ANG-SND5-"+ids.New(), "EUR", provTestOffer())
	o.WorkspaceID = wsID
	o.BuyerOrgID = &orgID
	created, err := offerStore.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := offerStore.Update(context.Background(), created.ID, wsID, map[string]any{"buyer_org_id": orgID}, 0); err != nil {
		t.Fatalf("set buyer_org_id: %v", err)
	}

	sent, err := offerStore.Send(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if sent.BuyerSnapshot["display_name"] != "Acme GmbH" {
		t.Fatalf("expected buyer_snapshot.display_name=Acme GmbH, got %+v", sent.BuyerSnapshot)
	}
}
