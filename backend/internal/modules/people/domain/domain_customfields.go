package domain

import "encoding/json"

type personAlias Person

// MarshalJSON flattens p.CustomFields onto the top-level wire object, each
// entry under its own cf_<slug> key, alongside the fixed fields.
func (p Person) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(personAlias(p))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(base, &fields); err != nil {
		return nil, err
	}
	for k, v := range p.CustomFields {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		fields[k] = raw
	}
	return json.Marshal(fields)
}

// UnmarshalJSON round-trips every fixed field. It deliberately does not
// populate CustomFields — nothing in this module decodes wire JSON back into
// a Person's custom-field values.
func (p *Person) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*personAlias)(p))
}
