// Package connector is the Tier-0 capture seam. Capture connectors normalize
// external content into NormalizedRecords and write them through Sink — the
// single write path crm-core owns (ADR-0022 capture-build boundary). This
// package is dependency-free: no River/Redis/DB imports.
package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// NaturalKey is the idempotency key for a captured record:
// (workspace is supplied by the writer) (SourceSystem, SourceID).
type NaturalKey struct {
	SourceSystem string // e.g. "email", "telegram", "coldstart-scrape"
	SourceID     string // provider-stable id, e.g. "<msg-id>"
}

// EntityRef is an activity_link target a record wants linked.
type EntityRef struct {
	EntityType string // "person" | "organization" | "deal" | "lead"
	NaturalKey NaturalKey
}

// NormalizedRecord is the pure output of Normalize — no I/O produced it.
type NormalizedRecord struct {
	Kind       string          // "activity" | "person" | "organization" | ...
	NaturalKey NaturalKey      // (SourceSystem, SourceID) — both non-empty
	Source     string          // "<source_system>:<source_id>" provenance string
	CapturedBy string          // "connector:<name>" — non-empty
	Payload    json.RawMessage // normalized fields
	Raw        json.RawMessage // re-parseable original payload (memory-first)
	Links      []EntityRef     // optional activity_link targets
}

// Validate enforces the mandatory-provenance contract before the record
// reaches the write path. A connector that returns an invalid record is a bug.
func (r NormalizedRecord) Validate() error {
	switch {
	case r.Kind == "":
		return fmt.Errorf("connector: NormalizedRecord.Kind is empty")
	case r.NaturalKey.SourceSystem == "":
		return fmt.Errorf("connector: NaturalKey.SourceSystem is empty")
	case r.NaturalKey.SourceID == "":
		return fmt.Errorf("connector: NaturalKey.SourceID is empty")
	case r.Source == "":
		return fmt.Errorf("connector: Source (provenance) is empty")
	case r.CapturedBy == "":
		return fmt.Errorf("connector: CapturedBy is empty")
	}
	return nil
}

// ErrSkip is returned by Normalize for an out-of-scope record (e.g. a
// personal-exclusion match). The pipeline drops it without error.
var ErrSkip = errors.New("connector: skip record")

// Descriptor identifies a connector and its governance posture.
type Descriptor struct {
	Name   string   // registry key, non-empty
	Scopes []string // must be ⊆ the granting human's scopes
	Tier   string   // risk tier for the governance surface
}

// Connector turns raw external bytes into normalized records.
type Connector interface {
	Descriptor() Descriptor
	// Normalize is PURE: no network, no DB. Returns records or ErrSkip.
	Normalize(ctx context.Context, raw []byte) ([]NormalizedRecord, error)
}

// Sink is the single write path. crm-core implements it; connectors never
// write rows themselves.
type Sink interface {
	Upsert(ctx context.Context, rec NormalizedRecord) error
}

var registry = map[string]Connector{}

// Register adds c to the global connector registry keyed by Descriptor().Name.
// It panics on an empty Name or empty Scopes (a registration bug) or a
// duplicate name. Called from init() in each connector package.
func Register(c Connector) {
	d := c.Descriptor()
	if d.Name == "" {
		panic("connector: Descriptor().Name is empty")
	}
	if len(d.Scopes) == 0 {
		panic("connector: Descriptor().Scopes is empty for " + d.Name)
	}
	if _, dup := registry[d.Name]; dup {
		panic("connector: duplicate registration for " + d.Name)
	}
	registry[d.Name] = c
}

// All returns every registered Connector.
func All() []Connector {
	out := make([]Connector, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	return out
}

// Get returns the connector registered under name, or false.
//
//nolint:ireturn // seam returns the Connector interface by design (registry lookup)
func Get(name string) (Connector, bool) { c, ok := registry[name]; return c, ok }

// HealthChecker is an OPTIONAL connector capability. An unhealthy connector
// must never fail core CRM reads (ACX.5) — the ops surface reports it and the
// pipeline degrades gracefully.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// ScopeSubset reports whether every scope in want is present in have (want ⊆ have).
func ScopeSubset(want, have []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, s := range have {
		set[s] = struct{}{}
	}
	for _, s := range want {
		if _, ok := set[s]; !ok {
			return false
		}
	}
	return true
}

// ErrScopeExceeded is the connector-seam sentinel for an over-scoped
// registration (agent ≤ human, 03b). It is local to this package — NOT errs.*
// — because the connector seam must stay dependency-free (arch-lint forbids
// Tier-0 from importing errs). A caller in crm-core/crm-capture may translate
// it to errs.ErrScopeExceeded at its own edge.
var ErrScopeExceeded = errors.New("connector: scopes exceed granting human")

// RegisterScoped registers c only if its declared scopes are a subset of the
// granting human's scopes (agent ≤ human, 03b). Over-scoped registration is a
// governance violation and is refused with ErrScopeExceeded.
func RegisterScoped(c Connector, humanScopes []string) error {
	d := c.Descriptor()
	if !ScopeSubset(d.Scopes, humanScopes) {
		return fmt.Errorf("%w: connector %q scopes %v exceed granting human's %v",
			ErrScopeExceeded, d.Name, d.Scopes, humanScopes)
	}
	Register(c)
	return nil
}

// HealthOf runs c.HealthCheck if c implements HealthChecker; a connector with
// no health probe is treated as healthy.
func HealthOf(ctx context.Context, c Connector) error {
	if hc, ok := c.(HealthChecker); ok {
		return hc.HealthCheck(ctx)
	}
	return nil
}
