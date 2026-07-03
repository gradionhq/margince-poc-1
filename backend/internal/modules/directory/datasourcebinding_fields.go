package crmcore

import "time"

// Field-map → domain-struct converters for the Datasource binding's Create/Update path.
// Each accepts either a ready-made domain value (passed straight through) or a
// generic map[string]any field patch, pulling the keys that entity supports and
// applying the same status/timestamp defaults the native store would.

func personFromFields(v any) Person {
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

func orgFromFields(v any) Organization {
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

func dealFromFields(v any) Deal {
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

func activityFromFields(v any) Activity {
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

func leadFromFields(v any) Lead {
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
