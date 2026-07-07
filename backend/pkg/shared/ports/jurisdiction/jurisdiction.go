// Package jurisdiction is the Tier-0 seam for country-specific behavior
// (ADR-0042). Packs (crm-de, ...) implement Pack and self-register via init();
// cmd/server selects which packs link at compile time. Core never imports a pack.
package jurisdiction

// seam: jurisdiction packs implement — frozen, additive-only (ADR-0042, doc 04)

import (
	"context"
	"io/fs"
	"sync"
)

// FormatID identifies an e-invoice format (e.g. "xrechnung", "zugferd").
type FormatID string

// ConformityArtifactID identifies a conformity artifact type (e.g. "cra-doc").
type ConformityArtifactID string

// TrustArtifactID identifies a trust artifact type (e.g. "bsi-c5", "tisax").
type TrustArtifactID string

// ExportProfileID identifies a jurisdiction-specific export bundle profile.
type ExportProfileID string

// Invoice is a seam-level e-invoice document for EN-16931 / XRechnung (B-E17.2).
// All money amounts are in ISO-4217 minor units (int64). Packs consume this type
// additively through FiscalFormatter.Emit/Validate.
type Invoice struct {
	// Document identity
	InvoiceNumber string // BT-1: invoice number
	IssueDate     string // BT-2: YYYY-MM-DD formatted issue date

	// Buyer / seller references
	BuyerReference string // BT-10: free-text buyer reference
	OrderReference string // BT-13: purchase order / offer reference

	// Currency (ISO 4217)
	Currency string // BT-5: invoice currency code

	// Tax category (single-category document in this implementation)
	TaxCategoryCode string // BT-118: VAT category code
	TaxRate         string // BT-119/BT-152: numeric string, never float

	// Document totals in minor units
	NetMinor   int64 // BT-109: tax-exclusive amount
	TaxMinor   int64 // BT-110: tax amount
	GrossMinor int64 // BT-112: tax-inclusive amount

	// Line items (BG-25)
	Lines []InvoiceLine
}

// InvoiceLine is one EN-16931 invoice line (BG-25 / BT-126..BT-136).
// All money amounts are in ISO-4217 minor units (int64).
type InvoiceLine struct {
	LineID                   string // BT-126: line identifier
	Description              string // BT-153: item name
	Quantity                 string // BT-129: invoiced quantity
	Unit                     string // BT-130: UN/ECE unit code
	LineExtensionAmountMinor int64  // BT-131: line net amount
	TaxCategoryCode          string // BT-151: line VAT category code
	TaxRate                  string // BT-152: line VAT rate, numeric string
}

// InboundDoc is a seam-level placeholder for an inbound supplier document (fleshed out additively by E17).
type InboundDoc struct{}

// Record is a seam-level placeholder for a document record subject to retention (fleshed out additively by E17).
type Record struct{}

// RetentionClass is a seam-level placeholder for a statutory retention classification result (fleshed out additively by E17).
type RetentionClass struct{}

// ForkBuild is a seam-level placeholder for the input to a conformity DoC build (fleshed out additively by E17).
type ForkBuild struct{}

// DoC is a seam-level placeholder for a Declaration of Conformity artifact (fleshed out additively by E17).
type DoC struct{}

// Pack is one jurisdiction's contribution surface. Registered by the pack's init();
// queried by core through For(code)/Applicable(country). Code is a free-form id —
// a country ("de") or a region ("eu"). NOTE: no Locales() — i18n is core (per-user, EP09).
type Pack interface {
	Code() string                 // ISO-3166-1 alpha-2, or a region id
	Fiscal() FiscalFormatter      // nil if the jurisdiction has no e-invoice mandate
	Retention() RetentionPolicy   // nil → core default retention only
	Conformity() ConformityRegime // nil → no conformity DoC; "de"/"eu" → CRA DoC regime
	TrustArtifacts() TrustArtifactSet
	ExportProfiles() []ExportProfile
	Migrations() fs.FS // embedded migrations/ FS, or nil if the pack ships no tables
}

// FiscalFormatter emits/parses/validates a jurisdiction's e-invoice formats.
type FiscalFormatter interface {
	Formats() []FormatID
	Emit(ctx context.Context, inv Invoice, f FormatID) ([]byte, error)
	Validate(ctx context.Context, doc []byte, f FormatID) error // schema + Schematron; refuse-to-emit
	Parse(ctx context.Context, doc []byte) (InboundDoc, error)  // inbound supplier document
}

// RetentionPolicy classifies a record into a statutory retention class (GoBD is the DE impl).
type RetentionPolicy interface {
	Classify(r Record) (RetentionClass, error) // window + immutability flag; ∅ → core default
}

// ConformityRegime declares which conformity artifacts a jurisdiction requires (e.g. the CRA DoC).
type ConformityRegime interface {
	Artifacts() []ConformityArtifactID // e.g. "cra-doc"
	BuildDoC(ctx context.Context, fork ForkBuild) (DoC, error)
}

// TrustArtifactSet is the set of trust descriptors a jurisdiction contributes (BSI C5, TISAX, …).
type TrustArtifactSet interface {
	Artifacts() []TrustArtifactID
}

// ExportProfile names a jurisdiction-specific export bundle profile (e.g. a GoBD export).
type ExportProfile interface {
	ProfileID() ExportProfileID
}

var (
	mu       sync.RWMutex
	registry = map[string]Pack{}
)

// countryRegions maps a country code to the region codes it belongs to (V1: de → eu).
var countryRegions = map[string][]string{
	"de": {"eu"},
}

// Register adds a pack (called from a pack's init()). Panics on a duplicate code.
func Register(code string, p Pack) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[code]; dup {
		panic("jurisdiction: duplicate registration for " + code)
	}
	registry[code] = p
}

// For returns the registered pack for a code, if its module is linked.
//
//nolint:ireturn // registry seam returns the Pack interface by design
func For(code string) (Pack, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[code]
	return p, ok
}

// Codes lists the linked jurisdiction codes.
func Codes() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for c := range registry {
		out = append(out, c)
	}
	return out
}

// Applicable returns the applicable set of packs for a country: the country's own pack
// plus any registered pack for a region the country belongs to. Unknown/unlinked country
// returns an empty slice. V1: Applicable("de") → [de].
func Applicable(country string) []Pack {
	mu.RLock()
	defer mu.RUnlock()
	var out []Pack
	if p, ok := registry[country]; ok {
		out = append(out, p)
	}
	for _, region := range countryRegions[country] {
		if p, ok := registry[region]; ok {
			out = append(out, p)
		}
	}
	return out
}
