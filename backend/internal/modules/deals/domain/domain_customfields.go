package domain

import "encoding/json"

type dealAlias Deal

// MarshalJSON flattens CustomFields onto the top-level JSON object under
// each field's cf_<slug> key (never nested), matching crm.yaml's
// additionalProperties: true + x-extension: true convention.
func (d Deal) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(dealAlias(d))
	if err != nil {
		return nil, err
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(base, &out); err != nil {
		return nil, err
	}
	for k, v := range d.CustomFields {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		out[k] = b
	}
	return json.Marshal(out)
}

// UnmarshalJSON is a plain pass-through to dealAlias — CustomFields is never
// populated by unmarshal, kept only so Deal round-trips its fixed fields
// through json.Marshal/Unmarshal in tests.
func (d *Deal) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*dealAlias)(d))
}
