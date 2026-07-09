// Package extraction is the Tier-0 seam for staged, evidence-grounded document
// field extraction (RD-T10). It mirrors the retrieval seam's no-op/fixture
// pattern: the production default is honestly empty until a real extractor
// exists.
package extraction

import "context"

// ExtractedField is one attempted grounded field, or one omitted field when
// Omitted is true.
type ExtractedField struct {
	Field         string
	Value         string
	SourceQuote   string
	PageOrSection string
	Confidence    string
	Omitted       bool
	OmittedReason string
}

// Extractor is the staged AI-extraction seam.
type Extractor interface {
	Extract(ctx context.Context, attachmentID string) ([]ExtractedField, error)
}

// NoOpExtractor is the production default.
type NoOpExtractor struct{}

// Extract returns no fields and no error because no production extractor exists.
func (NoOpExtractor) Extract(context.Context, string) ([]ExtractedField, error) {
	return nil, nil
}

// FixtureExtractor returns pre-seeded extraction rows keyed by attachment ID.
type FixtureExtractor struct {
	Fields map[string][]ExtractedField
}

// Extract returns the seeded rows for the attachment ID, or an empty result for
// an unknown attachment.
func (f FixtureExtractor) Extract(_ context.Context, attachmentID string) ([]ExtractedField, error) {
	if f.Fields == nil {
		return nil, nil
	}
	return f.Fields[attachmentID], nil
}
