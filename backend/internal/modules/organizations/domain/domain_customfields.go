package domain

import "encoding/json"

// organizationAlias breaks MarshalJSON's infinite-recursion loop.
type organizationAlias Organization

// MarshalJSON flattens CustomFields onto the top-level JSON object under each
// field's cf_<slug> key (crm.yaml additionalProperties: true + x-extension: true),
// never nested — marshal the alias for the fixed fields, overlay each custom
// value, re-marshal.
func (o Organization) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(organizationAlias(o))
	if err != nil {
		return nil, err
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(base, &out); err != nil {
		return nil, err
	}
	for k, v := range o.CustomFields {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		out[k] = raw
	}
	return json.Marshal(out)
}

// UnmarshalJSON is a plain pass-through to the alias (CustomFields is never
// populated by unmarshal; create/update extract cf_ values from the raw request
// body against the catalog's active-column list). Kept for round-trip symmetry.
func (o *Organization) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*organizationAlias)(o))
}
