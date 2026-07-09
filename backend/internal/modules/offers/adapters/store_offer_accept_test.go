//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

func acceptTestStore(db *sql.DB) *adapters.OfferStore {
	return adapters.NewOfferStore(db).WithDealStore(deals.NewDealStore(db))
}

func TestOfferStore_Accept_SentOffer_FlipsStatusSyncsDealAndEmitsPairedEvents(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := acceptTestStore(db)
	created := createTestOffer(t, db, dealID, wsID, "ANG-ACC-", "EUR")
	if _, err := db.Exec(`UPDATE offer SET status='sent', gross_minor=297500, net_minor=250000, tax_minor=47500 WHERE id=$1::uuid`, created.ID); err != nil {
		t.Fatalf("force sent + totals: %v", err)
	}

	accepted, err := s.Accept(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if accepted.Status != domain.OfferStatusAccepted {
		t.Fatalf("expected status=accepted, got %s", accepted.Status)
	}
	if accepted.AcceptedAt == nil {
		t.Fatal("expected accepted_at populated")
	}

	var amountMinor int64
	var currency string
	if err := db.QueryRow(`SELECT amount_minor, currency FROM deal WHERE id=$1::uuid`, dealID).Scan(&amountMinor, &currency); err != nil {
		t.Fatalf("read deal: %v", err)
	}
	if amountMinor != 297500 {
		t.Fatalf("expected deal.amount_minor=297500, got %d", amountMinor)
	}
	if currency != "EUR" {
		t.Fatalf("expected deal.currency=EUR, got %s", currency)
	}

	var offerCount int
	var offerPayload []byte
	if err := db.QueryRow(`SELECT count(*), payload FROM event_outbox WHERE topic='offer.accepted' AND entity_id=$1::uuid GROUP BY payload`, created.ID).Scan(&offerCount, &offerPayload); err != nil {
		t.Fatalf("read offer event: %v", err)
	}
	if offerCount != 1 {
		t.Fatalf("expected exactly one offer.accepted event, got %d", offerCount)
	}
	var dealCount int
	var dealPayload []byte
	if err := db.QueryRow(`SELECT count(*), payload FROM event_outbox WHERE topic='deal.updated' AND entity_id=$1::uuid GROUP BY payload`, dealID).Scan(&dealCount, &dealPayload); err != nil {
		t.Fatalf("read deal event: %v", err)
	}
	if dealCount != 1 {
		t.Fatalf("expected exactly one deal.updated event, got %d", dealCount)
	}

	var offerBody, dealBody struct {
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.Unmarshal(offerPayload, &offerBody); err != nil {
		t.Fatalf("unmarshal offer payload: %v", err)
	}
	if err := json.Unmarshal(dealPayload, &dealBody); err != nil {
		t.Fatalf("unmarshal deal payload: %v", err)
	}
	if offerBody.CorrelationID == "" || offerBody.CorrelationID != dealBody.CorrelationID {
		t.Fatalf("expected shared non-empty correlation_id, got offer=%q deal=%q", offerBody.CorrelationID, dealBody.CorrelationID)
	}

	var offerAuditCount, dealAuditCount int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity_type='offer' AND entity_id=$1::uuid`, created.ID).Scan(&offerAuditCount); err != nil {
		t.Fatalf("count offer audit: %v", err)
	}
	if offerAuditCount != 1 {
		t.Fatalf("expected exactly one offer audit row, got %d", offerAuditCount)
	}
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity_type='deal' AND entity_id=$1::uuid`, dealID).Scan(&dealAuditCount); err != nil {
		t.Fatalf("count deal audit: %v", err)
	}
	if dealAuditCount != 1 {
		t.Fatalf("expected exactly one deal audit row, got %d", dealAuditCount)
	}
}

func TestOfferStore_Accept_NotSent_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := acceptTestStore(db)
	created := createTestOffer(t, db, dealID, wsID, "ANG-ACC2-", "EUR")

	_, err := s.Accept(context.Background(), created.ID, wsID)
	if !errors.Is(err, adapters.ErrOfferNotAcceptable) {
		t.Fatalf("expected ErrOfferNotAcceptable for a draft offer, got %v", err)
	}

	var dealEvt int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='deal.updated' AND entity_id=$1::uuid`, dealID).Scan(&dealEvt); err != nil {
		t.Fatalf("count deal.updated: %v", err)
	}
	if dealEvt != 0 {
		t.Fatalf("expected zero deal.updated events from a rejected accept, got %d", dealEvt)
	}
}

func TestOfferStore_Accept_AlreadyAccepted_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := acceptTestStore(db)
	created := createTestOffer(t, db, dealID, wsID, "ANG-ACC3-", "EUR")
	if _, err := db.Exec(`UPDATE offer SET status='sent' WHERE id=$1::uuid`, created.ID); err != nil {
		t.Fatalf("force sent: %v", err)
	}
	if _, err := s.Accept(context.Background(), created.ID, wsID); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	_, err := s.Accept(context.Background(), created.ID, wsID)
	if !errors.Is(err, adapters.ErrOfferNotAcceptable) {
		t.Fatalf("expected ErrOfferNotAcceptable on a second accept, got %v", err)
	}
}
