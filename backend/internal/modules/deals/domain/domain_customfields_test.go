package domain

import (
	"encoding/json"
	"testing"
)

func TestDealMarshalJSONFlattensCustomFields(t *testing.T) {
	d := Deal{
		ID:     "deal-1",
		Name:   "Acme",
		Status: "open",
		CustomFields: map[string]any{
			"cf_deal_score": 42,
			"cf_region":     "emea",
		},
	}

	b, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["cf_deal_score"] != float64(42) {
		t.Fatalf("cf_deal_score = %v, want 42", out["cf_deal_score"])
	}
	if out["cf_region"] != "emea" {
		t.Fatalf("cf_region = %v, want emea", out["cf_region"])
	}
}

func TestDealUnmarshalJSONKeepsFixedFields(t *testing.T) {
	var d Deal
	if err := d.UnmarshalJSON([]byte(`{"id":"deal-1","name":"Acme","status":"open"}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if d.ID != "deal-1" || d.Name != "Acme" || d.Status != "open" {
		t.Fatalf("decoded deal = %+v", d)
	}
}
