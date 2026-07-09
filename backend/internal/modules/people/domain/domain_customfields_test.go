package domain

import (
	"encoding/json"
	"testing"
)

func TestPerson_MarshalJSON_FlattensCustomFields(t *testing.T) {
	score := int64(42)
	p := Person{
		ID:           "person-1",
		WorkspaceID:  "ws-1",
		FullName:     "Ada Lovelace",
		CustomFields: map[string]any{"cf_score": score},
	}

	got, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(got, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := body["CustomFields"]; ok {
		t.Fatalf("CustomFields key leaked into JSON: %s", got)
	}
	if body["cf_score"] != float64(score) {
		t.Fatalf("cf_score = %#v, want %d", body["cf_score"], score)
	}
	if body["full_name"] != "Ada Lovelace" {
		t.Fatalf("full_name = %#v, want %q", body["full_name"], "Ada Lovelace")
	}
}

func TestPerson_MarshalJSON_NilAndEmptyCustomFieldsMatch(t *testing.T) {
	base := Person{ID: "person-1", WorkspaceID: "ws-1", FullName: "Ada Lovelace"}
	empty := base
	empty.CustomFields = map[string]any{}

	gotNil, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("MarshalJSON nil: %v", err)
	}
	gotEmpty, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("MarshalJSON empty: %v", err)
	}
	if string(gotNil) != string(gotEmpty) {
		t.Fatalf("nil and empty custom fields diverged:\nnil:   %s\nempty: %s", gotNil, gotEmpty)
	}
}

func TestPerson_UnmarshalJSON_RoundTripsFixedFieldsOnly(t *testing.T) {
	in := Person{
		ID:          "person-1",
		WorkspaceID: "ws-1",
		FirstName:   stringPtr("Ada"),
		LastName:    stringPtr("Lovelace"),
		FullName:    "Ada Lovelace",
		Title:       stringPtr("Mathematician"),
		OwnerID:     stringPtr("owner-1"),
		Social:      map[string]any{"linkedin": "ada"},
		Address:     map[string]any{"city": "London"},
		Version:     7,
		Source:      "test",
		CapturedBy:  "human:test",
	}

	gotJSON, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var out Person
	if err := json.Unmarshal(gotJSON, &out); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if out.ID != in.ID || out.WorkspaceID != in.WorkspaceID || out.FullName != in.FullName || out.Version != in.Version {
		t.Fatalf("fixed fields did not round-trip:\n in=%+v\nout=%+v", in, out)
	}
	if out.CustomFields != nil {
		t.Fatalf("CustomFields unexpectedly populated: %#v", out.CustomFields)
	}
}

func stringPtr(s string) *string { return &s }
