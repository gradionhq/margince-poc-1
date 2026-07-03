package crmcore

import "testing"

func TestOrgStrength_PlainMax(t *testing.T) {
	people := []OrgStrengthInput{
		{PersonID: "p1", Strength: &StrengthResult{Score: 47}},
		{PersonID: "p2", Strength: &StrengthResult{Score: 82}},
		{PersonID: "p3", Strength: &StrengthResult{Score: 10}},
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
		{PersonID: "p2", Strength: &StrengthResult{Score: 5}},
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
		{PersonID: "zz-older", Strength: &StrengthResult{Score: 50}, LastInteraction: older},
		{PersonID: "aa-newer", Strength: &StrengthResult{Score: 50}, LastInteraction: newer},
	}
	_, topID, _ := OrgStrength(people)
	if topID != "aa-newer" {
		t.Errorf("topID = %q, want aa-newer (more recent last interaction wins over lower id)", topID)
	}

	people2 := []OrgStrengthInput{
		{PersonID: "zz-id", Strength: &StrengthResult{Score: 50}, LastInteraction: newer},
		{PersonID: "aa-id", Strength: &StrengthResult{Score: 50}, LastInteraction: newer},
	}
	_, topID2, _ := OrgStrength(people2)
	if topID2 != "aa-id" {
		t.Errorf("topID = %q, want aa-id (lowest id, final tie-break)", topID2)
	}
}
