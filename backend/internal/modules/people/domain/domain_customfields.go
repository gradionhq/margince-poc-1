package domain

import "encoding/json"

type personAlias Person

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

func (p *Person) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*personAlias)(p))
}
