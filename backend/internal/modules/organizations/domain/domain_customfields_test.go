package domain

import (
	"encoding/json"
	"testing"
)

func TestOrganizationMarshalJSONFlattensCustomFields(t *testing.T) {
	o := Organization{
		ID:           "org-1",
		DisplayName:  "Acme",
		CustomFields: map[string]any{"cf_score": 12, "cf_note": "alpha"},
	}
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal wire json: %v", err)
	}
	if _, ok := got["CustomFields"]; ok {
		t.Fatalf("CustomFields leaked into wire json: %v", got)
	}
	if got["cf_score"].(float64) != 12 {
		t.Fatalf("cf_score = %v, want 12", got["cf_score"])
	}
	if got["cf_note"].(string) != "alpha" {
		t.Fatalf("cf_note = %v, want alpha", got["cf_note"])
	}
}

func TestOrganizationUnmarshalJSONPreservesFixedFields(t *testing.T) {
	var got Organization
	if err := json.Unmarshal([]byte(`{"id":"org-1","display_name":"Acme"}`), &got); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if got.ID != "org-1" || got.DisplayName != "Acme" {
		t.Fatalf("unexpected organization: %+v", got)
	}
}
