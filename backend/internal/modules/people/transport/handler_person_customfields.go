package transport

import (
	"encoding/json"

	activities "github.com/gradionhq/margince/backend/internal/modules/activities"
	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	peopledomain "github.com/gradionhq/margince/backend/internal/modules/people/domain"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
)

// personDetailResponse is the person-360 composite read — the person itself
// plus relationships, deals, and activities. Its own Relationships/Deals/
// Activities fields shadow the embedded Person's `omitempty`-tagged fields of
// the same Go field name (same class as deals/transport's dealDetailResponse:
// list responses must omit these keys when unset, but a single-record read
// must always show `[]`, never `null` or absent, when the composite result
// set is legitimately empty — not expressible via one struct/tag serving both
// list and get semantics).
type personDetailResponse struct {
	peopledomain.Person
	Relationships []relationships.Relationship `json:"relationships"`
	Deals         []deals.Deal                 `json:"deals"`
	Activities    []activities.ActivityRef     `json:"activities"`
}

// MarshalJSON is required because Person now defines its own MarshalJSON
// (to flatten CustomFields onto the wire object) — without this override,
// Go's method promotion would make personDetailResponse silently inherit
// Person's MarshalJSON, dropping relationships/deals/activities from
// GET /people/{id}.
func (r personDetailResponse) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(r.Person)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(base, &fields); err != nil {
		return nil, err
	}
	for k, v := range map[string]any{
		"relationships": r.Relationships,
		"deals":         r.Deals,
		"activities":    r.Activities,
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		fields[k] = raw
	}
	return json.Marshal(fields)
}

// extractCustomFields pulls each active cf_* key present in raw into a
// map[string]any, best-effort (a key with a malformed shape is simply
// omitted — no per-key contract schema exists for an extension property).
func extractCustomFields(raw map[string]json.RawMessage, active []string) map[string]any {
	out := map[string]any{}
	for _, key := range active {
		v, ok := raw[key]
		if !ok {
			continue
		}
		var decoded any
		if err := json.Unmarshal(v, &decoded); err != nil {
			continue
		}
		out[key] = decoded
	}
	return out
}
