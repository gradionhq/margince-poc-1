// org_strength_test.go — ported from modules/directory/org_strength_test.go
// (package crmcore → package domain; StrengthResult → strength.Result,
// OrgStrengthInput.Strength *StrengthResult → *strength.Result).
package domain

import (
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/strength"
)

// fixedStrengthClock is TEST-DET-1's pinned test clock for PO-F-3/PO-N-ORGSTRENGTH,
// matching modules/directory/strength_test.go's constant so results are stable
// across packages.
var fixedStrengthClock = time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)

func TestOrgStrength_PlainMax(t *testing.T) {
	people := []OrgStrengthInput{
		{PersonID: "p1", Strength: &strength.Result{Score: 47}},
		{PersonID: "p2", Strength: &strength.Result{Score: 82}},
		{PersonID: "p3", Strength: &strength.Result{Score: 10}},
	}
	score, topID, hasSignal := OrgStrength(people)
	if !hasSignal {
		t.Fatal("want hasSignal=true")
	}
	if score != 82 {
		t.Errorf("score = %d, want 82 (plain max, no cap/normalize)", score)
	}
	if topID != "p2" {
		t.Errorf("topID = %q, want p2", topID)
	}
}

func TestOrgStrength_ExcludesNoSignalYet(t *testing.T) {
	people := []OrgStrengthInput{
		{PersonID: "p1", Strength: nil},
		{PersonID: "p2", Strength: &strength.Result{Score: 5}},
	}
	score, topID, hasSignal := OrgStrength(people)
	if !hasSignal || score != 5 || topID != "p2" {
		t.Errorf("got score=%d topID=%q hasSignal=%v, want 5/p2/true", score, topID, hasSignal)
	}
}

func TestOrgStrength_AllNoSignalYet(t *testing.T) {
	people := []OrgStrengthInput{{PersonID: "p1", Strength: nil}, {PersonID: "p2", Strength: nil}}
	_, _, hasSignal := OrgStrength(people)
	if hasSignal {
		t.Error("want hasSignal=false when every person is no-signal-yet")
	}
}

// TestOrgStrength_TieBreak: highest strength -> most recent last interaction
// -> lowest person id.
func TestOrgStrength_TieBreak(t *testing.T) {
	older := fixedStrengthClock.AddDate(0, 0, -10)
	newer := fixedStrengthClock.AddDate(0, 0, -1)

	people := []OrgStrengthInput{
		{PersonID: "zz-older", Strength: &strength.Result{Score: 50}, LastInteraction: older},
		{PersonID: "aa-newer", Strength: &strength.Result{Score: 50}, LastInteraction: newer},
	}
	_, topID, _ := OrgStrength(people)
	if topID != "aa-newer" {
		t.Errorf("topID = %q, want aa-newer (more recent last interaction wins over lower id)", topID)
	}

	people2 := []OrgStrengthInput{
		{PersonID: "zz-id", Strength: &strength.Result{Score: 50}, LastInteraction: newer},
		{PersonID: "aa-id", Strength: &strength.Result{Score: 50}, LastInteraction: newer},
	}
	_, topID2, _ := OrgStrength(people2)
	if topID2 != "aa-id" {
		t.Errorf("topID = %q, want aa-id (lowest id, final tie-break)", topID2)
	}
}
