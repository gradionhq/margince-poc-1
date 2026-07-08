package adapters

import (
	"bytes"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
)

func testOfferForPDF() domain.Offer {
	return domain.Offer{
		ID:          "offer-1",
		WorkspaceID: "workspace-1",
		DealID:      "deal-1",
		OfferNumber: "ANG-001",
		Revision:    2,
		Status:      domain.OfferStatusDraft,
		Currency:    "EUR",
		NetMinor:    123456,
		TaxMinor:    23456,
		GrossMinor:  146912,
		CreatedAt:   time.Date(2026, 7, 8, 9, 10, 11, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 7, 8, 9, 10, 11, 0, time.UTC),
	}
}

func TestRenderOfferPDF_IncludesOfferDataAndTotals(t *testing.T) {
	o := testOfferForPDF()
	lineItems := []domain.OfferLineItem{
		{
			ID:             "li-1",
			OfferID:        o.ID,
			Position:       1,
			Description:    "Consulting Day",
			Unit:           "day",
			Quantity:       2,
			UnitPriceMinor: 50000,
			TaxRate:        19,
		},
		{
			ID:             "li-2",
			OfferID:        o.ID,
			Position:       2,
			Description:    "Setup Fee",
			Unit:           "flat",
			Quantity:       1,
			UnitPriceMinor: 23456,
			TaxRate:        19,
		},
	}
	buyerBlock := map[string]any{
		"organization_id": "org-1",
		"display_name":    "Acme GmbH",
		"address":         "Main St 1, Berlin",
	}

	pdf, err := RenderOfferPDF(o, lineItems, buyerBlock, "Margince GmbH", "de-DE")
	if err != nil {
		t.Fatalf("RenderOfferPDF() error = %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("RenderOfferPDF() returned empty PDF")
	}

	mustContain := []string{
		"ANG-001",
		"Revision 2",
		"Acme GmbH",
		"Main St 1, Berlin",
		"Margince GmbH",
		"Consulting Day",
		"2 x 500.00 EUR",
		"Setup Fee",
		"234.56 EUR",
		"1234.56 EUR",
		"234.56 EUR",
		"1469.12 EUR",
	}
	for _, needle := range mustContain {
		if !bytes.Contains(pdf, []byte(needle)) {
			t.Fatalf("PDF missing %q\n%s", needle, pdf)
		}
	}
	if got := formatMinor(146912, "EUR"); got != "1469.12 EUR" {
		t.Fatalf("formatMinor() = %q, want %q", got, "1469.12 EUR")
	}
}

func TestRenderOfferPDF_LocaleDrivesLabels(t *testing.T) {
	o := testOfferForPDF()
	lineItems := []domain.OfferLineItem{
		{
			ID:             "li-1",
			OfferID:        o.ID,
			Position:       1,
			Description:    "Consulting Day",
			Unit:           "day",
			Quantity:       2,
			UnitPriceMinor: 50000,
			TaxRate:        19,
		},
	}
	buyerBlock := map[string]any{
		"organization_id": "org-1",
		"display_name":    "Acme GmbH",
		"address":         "Main St 1, Berlin",
	}

	dePDF, err := RenderOfferPDF(o, lineItems, buyerBlock, "Margince GmbH", "de-DE")
	if err != nil {
		t.Fatalf("RenderOfferPDF(de-DE) error = %v", err)
	}
	enPDF, err := RenderOfferPDF(o, lineItems, buyerBlock, "Margince GmbH", "en")
	if err != nil {
		t.Fatalf("RenderOfferPDF(en) error = %v", err)
	}

	if !bytes.Contains(dePDF, []byte("Angebot")) {
		t.Fatalf("de-DE PDF missing %q\n%s", "Angebot", dePDF)
	}
	if !bytes.Contains(dePDF, []byte("Nettobetrag")) {
		t.Fatalf("de-DE PDF missing %q\n%s", "Nettobetrag", dePDF)
	}
	if bytes.Contains(dePDF, []byte("Offer ")) {
		t.Fatalf("de-DE PDF unexpectedly contains English label %q\n%s", "Offer ", dePDF)
	}

	if !bytes.Contains(enPDF, []byte("Offer ")) {
		t.Fatalf("en PDF missing %q\n%s", "Offer ", enPDF)
	}
	if !bytes.Contains(enPDF, []byte("Net: ")) {
		t.Fatalf("en PDF missing %q\n%s", "Net: ", enPDF)
	}
	if bytes.Contains(enPDF, []byte("Angebot")) {
		t.Fatalf("en PDF unexpectedly contains German label %q\n%s", "Angebot", enPDF)
	}
}
