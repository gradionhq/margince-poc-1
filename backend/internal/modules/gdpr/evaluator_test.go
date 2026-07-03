package crmgdpr

import "testing"

func TestIsUnconverted(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"new", true},
		{"working", true},
		{"disqualified", true},
		{"promoted", false},
	}
	for _, c := range cases {
		if got := isUnconverted(c.status); got != c.want {
			t.Errorf("isUnconverted(%q) = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestIsLostDeal(t *testing.T) {
	if !isLostDeal("lost") {
		t.Error("isLostDeal(lost) = false, want true")
	}
	if isLostDeal("open") {
		t.Error("isLostDeal(open) = true, want false")
	}
	if isLostDeal("won") {
		t.Error("isLostDeal(won) = true, want false")
	}
}

func TestNonPersonEraseSupported(t *testing.T) {
	// Only activities are erasable on the non-person erase path. A lead/deal erase
	// must be refused so applyAction never writes a success-claiming audit row for
	// a query that touched zero rows.
	if !nonPersonEraseSupported("activity") {
		t.Error("activity must be erasable on the non-person erase path")
	}
	for _, ot := range []string{"lead", "deal", "organization", "person", ""} {
		if nonPersonEraseSupported(ot) {
			t.Errorf("object_type %q must NOT be erasable on the non-person path (would write a dishonest audit row)", ot)
		}
	}
}

func TestIsTranscript(t *testing.T) {
	if !isTranscript("transcript") {
		t.Error("isTranscript(transcript) = false, want true")
	}
	if isTranscript("gmail") {
		t.Error("isTranscript(gmail) = true, want false")
	}
	if isTranscript("") {
		t.Error("isTranscript('') = true, want false")
	}
}
