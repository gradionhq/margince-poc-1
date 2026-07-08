package adapters

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/go-pdf/fpdf"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
)

func formatMinor(minor int64, currency string) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	major := minor / 100
	frac := minor % 100
	return fmt.Sprintf("%s%d.%02d %s", sign, major, frac, currency)
}

func formatLineQuantity(quantity float64) string {
	return strconv.FormatFloat(quantity, 'f', -1, 64)
}

func buyerBlockString(buyerBlock map[string]any, key string) string {
	v, _ := buyerBlock[key].(string)
	return v
}

// pdfLabels holds the locale-specific label strings used in the rendered PDF.
type pdfLabels struct {
	title     string
	buyer     string
	lineItems string
	net       string
	tax       string
	gross     string
}

// resolvePDFLabels maps a locale to its label set. de-DE (and the empty
// string, matching OfferTemplate's own default) get German labels;
// everything else gets English labels.
func resolvePDFLabels(locale string) pdfLabels {
	if locale == "de-DE" || locale == "" {
		return pdfLabels{
			title:     "Angebot",
			buyer:     "Kunde",
			lineItems: "Positionen",
			net:       "Nettobetrag",
			tax:       "MwSt",
			gross:     "Gesamtbetrag",
		}
	}
	return pdfLabels{
		title:     "Offer",
		buyer:     "Buyer",
		lineItems: "Line items",
		net:       "Net",
		tax:       "Tax",
		gross:     "Total",
	}
}

// RenderOfferPDF builds the branded offer PDF from persisted offer data.
func RenderOfferPDF(o domain.Offer, lineItems []domain.OfferLineItem, buyerBlock map[string]any, issuerName, locale string) ([]byte, error) {
	labels := resolvePDFLabels(locale)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetMargins(16, 16, 16)
	pdf.SetAutoPageBreak(true, 16)
	pdf.SetTitle(labels.title+" "+o.OfferNumber, false)
	pdf.SetCreator("margince", false)
	pdf.SetAuthor(issuerName, false)
	pdf.SetSubject(labels.title+" PDF", false)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 18)
	pdf.Cell(0, 8, labels.title+" "+o.OfferNumber)
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 11)
	pdf.Cell(0, 6, "Revision "+strconv.FormatInt(o.Revision, 10))
	pdf.Ln(7)
	pdf.Cell(0, 6, "Issuer: "+issuerName)
	pdf.Ln(10)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 6, labels.buyer)
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 11)
	if id := buyerBlockString(buyerBlock, "organization_id"); id != "" {
		pdf.Cell(0, 6, "Organization ID: "+id)
		pdf.Ln(6)
	}
	if displayName := buyerBlockString(buyerBlock, "display_name"); displayName != "" {
		pdf.Cell(0, 6, displayName)
		pdf.Ln(6)
	}
	if address := buyerBlockString(buyerBlock, "address"); address != "" {
		pdf.MultiCell(0, 6, address, "", "L", false)
	}
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 6, labels.lineItems)
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 10)
	for _, li := range lineItems {
		pdf.Cell(0, 5, fmt.Sprintf("%d. %s", li.Position, li.Description))
		pdf.Ln(5)
		pdf.Cell(0, 5, fmt.Sprintf("%s x %s", formatLineQuantity(li.Quantity), formatMinor(li.UnitPriceMinor, o.Currency)))
		pdf.Ln(5)
	}
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 6, "Totals")
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 11)
	pdf.Cell(0, 6, labels.net+": "+formatMinor(o.NetMinor, o.Currency))
	pdf.Ln(6)
	pdf.Cell(0, 6, labels.tax+": "+formatMinor(o.TaxMinor, o.Currency))
	pdf.Ln(6)
	pdf.Cell(0, 6, labels.gross+": "+formatMinor(o.GrossMinor, o.Currency))
	pdf.Ln(6)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
