package domain

import "encoding/json"

type organizationAlias Organization

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

func (o *Organization) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*organizationAlias)(o))
}
