package crmgdpr

import (
	"testing"
	"time"
)

func TestOverAge_Boundary(t *testing.T) {
	asOf := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	retainDays := 365

	// Exactly N days ago: not over age.
	exactly := asOf.Add(-time.Duration(retainDays) * 24 * time.Hour)
	if OverAge(asOf, retainDays, exactly) {
		t.Error("OverAge: exactly N days should NOT be over age")
	}

	// One second past N days: over age.
	justOver := exactly.Add(-time.Second)
	if !OverAge(asOf, retainDays, justOver) {
		t.Error("OverAge: N days + 1 second should be over age")
	}
}

func TestSeedDefaults_ProducesExactlyFiveRows(t *testing.T) {
	rows := defaultPolicies()
	if len(rows) != 5 {
		t.Fatalf("defaultPolicies: want 5 rows, got %d", len(rows))
	}

	type key struct{ objectType, category string }
	want := map[key]struct {
		retainDays int
		action     string
	}{
		{"lead", "unconverted"}:          {365, "anonymize"},
		{"activity", ""}:                 {1095, "archive"},
		{"activity", "transcript"}:       {365, "erase"},
		{"person", "no_consent_no_deal"}: {730, "anonymize"},
		{"deal", "lost"}:                 {1825, "archive"},
	}

	for _, r := range rows {
		k := key{r.ObjectType, r.Category}
		w, ok := want[k]
		if !ok {
			t.Errorf("unexpected row: object_type=%q category=%q", r.ObjectType, r.Category)
			continue
		}
		if r.RetainDays != w.retainDays {
			t.Errorf("row %v/%v: retain_days want %d got %d", r.ObjectType, r.Category, w.retainDays, r.RetainDays)
		}
		if r.Action != w.action {
			t.Errorf("row %v/%v: action want %q got %q", r.ObjectType, r.Category, w.action, r.Action)
		}
		delete(want, k)
	}
	for k := range want {
		t.Errorf("missing expected row: object_type=%q category=%q", k.objectType, k.category)
	}
}
