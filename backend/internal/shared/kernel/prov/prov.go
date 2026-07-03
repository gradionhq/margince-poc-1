// Package prov is the Tier-0 provenance kernel: who/what wrote a value
// (ADR-0006 capture trust tiers; never overwrite a human value silently).
package prov

// Provenance records the origin of a captured value.
type Provenance struct {
	CapturedBy string // e.g. "agent:logo-resolve", "human"
	Source     string // e.g. "og:image", "gmail", "website"
}

// ByHuman reports whether the value was entered by a person.
func (p Provenance) ByHuman() bool { return p.CapturedBy == "human" }
