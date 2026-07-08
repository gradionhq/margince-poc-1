package domain

import "time"

// Field-map → domain-struct converters for the datasource binding's Create/Update path.
// Each accepts either a ready-made domain value (passed straight through) or a
// generic map[string]any field patch, pulling the keys that entity supports and
// applying the same status/timestamp defaults the native store would.

// Column and field name constants used by the converters.
const (
	colFullName    = "full_name"
	colDisplayName = "display_name"
	fieldStatus    = "status"
	statusOpen     = "open"
	statusNew      = "new"
)

// toMap coerces v to map[string]any; returns an empty map for nil or non-map inputs.
func toMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// PersonFromFields converts a field map (or a ready-made Person) to a Person.
func PersonFromFields(v any) Person {
	if p, ok := v.(Person); ok {
		return p
	}
	m := toMap(v)
	var p Person
	if fn, ok := m[colFullName].(string); ok {
		p.FullName = fn
	}
	return p
}

// OrgFromFields converts a field map (or a ready-made Organization) to an Organization.
func OrgFromFields(v any) Organization {
	if o, ok := v.(Organization); ok {
		return o
	}
	m := toMap(v)
	var o Organization
	if dn, ok := m[colDisplayName].(string); ok {
		o.DisplayName = dn
	}
	return o
}

// DealFromFields converts a field map (or a ready-made Deal) to a Deal.
func DealFromFields(v any) Deal {
	if d, ok := v.(Deal); ok {
		return d
	}
	m := toMap(v)
	var d Deal
	if name, ok := m["name"].(string); ok {
		d.Name = name
	}
	if pid, ok := m["pipeline_id"].(string); ok {
		d.PipelineID = pid
	}
	if sid, ok := m["stage_id"].(string); ok {
		d.StageID = sid
	}
	d.Status = statusOpen
	return d
}

// ActivityFromFields converts a field map (or a ready-made Activity) to an Activity.
func ActivityFromFields(v any) Activity {
	if a, ok := v.(Activity); ok {
		return a
	}
	m := toMap(v)
	var a Activity
	if kind, ok := m["kind"].(string); ok {
		a.Kind = kind
	}
	a.OccurredAt = time.Now().UTC()
	return a
}

// LeadFromFields converts a field map (or a ready-made Lead) to a Lead.
func LeadFromFields(v any) Lead {
	if l, ok := v.(Lead); ok {
		return l
	}
	m := toMap(v)
	var l Lead
	if status, ok := m[fieldStatus].(string); ok {
		l.Status = status
	} else {
		l.Status = statusNew
	}
	return l
}

// ToMapValue coerces v to map[string]any for use by the provider's Update path.
// Returns an empty map for nil or non-map inputs.
func ToMapValue(v any) map[string]any { return toMap(v) }
