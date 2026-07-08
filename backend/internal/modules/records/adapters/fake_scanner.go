package adapters

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/records/ports"
)

// FakeScanner is a safe dev/test double for ports.Scanner — no real scanning
// product is integrated anywhere in this codebase (out of scope). It always
// returns the fixed result it was constructed with, demonstrating the
// injection seam without pretending to scan anything.
type FakeScanner struct{ Result string }

// NewFakeScanner returns a Scanner that always reports result (must be
// domain.ScanStatusClean or domain.ScanStatusBlocked — callers driving
// scanning verdicts in tests choose which).
func NewFakeScanner(result string) *FakeScanner { return &FakeScanner{Result: result} }

var _ ports.Scanner = (*FakeScanner)(nil)

func (f *FakeScanner) Scan(_ context.Context, _ string) (string, error) {
	return f.Result, nil
}
