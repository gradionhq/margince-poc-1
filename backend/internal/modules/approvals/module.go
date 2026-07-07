// Package crmapprovals owns the approval-inbox for staged 🟡 MCP tool actions.
//
// WS-E-a structural migration note: the domain/, ports/, adapters/, and app/
// subdirectory scaffold is established here. All implementation currently lives
// in this package's root files and will be progressively migrated once the
// internal tests that reference private symbols (parseToken) are updated to
// the external-test pattern. The adapters/ directory is the intended future home
// for all DB-backed code.
package crmapprovals
