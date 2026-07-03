// Package crmde is the German jurisdiction pack (ADR-0042): XRechnung/ZUGFeRD,
// DATEV, GoBD, eIDAS, de-DE locale. Own go.mod; implements the Tier-0
// jurisdiction seam and self-registers. Core never imports this module —
// only cmd/server links it (the compile-time switch).
//
// This is a pruned, empty pack shell (skeleton harvest): the DE feature code
// (DATEV/XRechnung/ZUGFeRD formatters, migrations) has been removed. Only the
// self-registration seam survives, so the jurisdiction registry + compile-time
// switch (cmd/server/imports_juris.go) keep their teeth — a future task
// reintroduces the real fiscal/export/migration implementations behind this
// same Pack.
//
// JURISDICTION: de
package crmde

import (
	"io/fs"

	"github.com/gradionhq/margince/backend/pkg/jurisdiction"
)

type pack struct{}

func (pack) Code() string { return "de" }

// Fiscal returns nil — DE fiscal formatters (XRechnung/ZUGFeRD) are pruned
// from this shell; a jurisdiction with no e-invoice mandate returns nil too.
//
//nolint:ireturn // seam returns the jurisdiction.FiscalFormatter interface by design (Pack contract)
func (pack) Fiscal() jurisdiction.FiscalFormatter { return nil }

// Retention returns nil — GoBD not implemented in this shell.
//
//nolint:ireturn // seam returns the jurisdiction.RetentionPolicy interface by design (Pack contract)
func (pack) Retention() jurisdiction.RetentionPolicy { return nil }

// Conformity returns nil — CRA DoC not implemented in this shell.
//
//nolint:ireturn // seam returns the jurisdiction.ConformityRegime interface by design (Pack contract)
func (pack) Conformity() jurisdiction.ConformityRegime { return nil }

// TrustArtifacts returns an empty set — BSI C5/TISAX not implemented in this shell.
//
//nolint:ireturn // seam returns the jurisdiction.TrustArtifactSet interface by design (Pack contract)
func (pack) TrustArtifacts() jurisdiction.TrustArtifactSet { return deTrustSet{} }

// ExportProfiles returns no profiles — DATEV EXTF export is pruned from this shell.
func (pack) ExportProfiles() []jurisdiction.ExportProfile { return nil }

// Migrations returns nil — this shell ships no tables of its own.
func (pack) Migrations() fs.FS { return nil }

// deTrustSet is an empty trust artifact set.
type deTrustSet struct{}

func (deTrustSet) Artifacts() []jurisdiction.TrustArtifactID { return nil }

func init() { jurisdiction.Register("de", pack{}) }
