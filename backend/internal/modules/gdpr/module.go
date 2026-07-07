// Package crmgdpr implements GDPR engine: consent, retention, erasure and SAR.
//
// WS-E-a structural migration note: the domain/, ports/, adapters/, and app/
// subdirectory scaffold is established here. All implementation currently lives
// in this package's root files and will be progressively migrated once the
// internal tests that reference private symbols (isUnconverted, isLostDeal,
// nonPersonEraseSupported, isTranscript, buildErasureTombstone) are updated to
// the external-test pattern. The adapters/ directory is the intended future home
// for all DB-backed code.
package crmgdpr
