package domain

import "encoding/json"

type dealAlias Deal

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

func (d *Deal) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*dealAlias)(d))
}
