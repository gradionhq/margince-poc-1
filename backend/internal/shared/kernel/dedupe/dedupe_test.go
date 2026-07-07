package dedupe

import "testing"

func TestJaroWinkler_GoldenExample(t *testing.T) {
	got := JaroWinkler("jon doe", "john doe")
	want := 0.9667
	if diff := got - want; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("JaroWinkler(%q,%q) = %v, want %v", "jon doe", "john doe", got, want)
	}
}

func TestNormalizeName_CasefoldUnaccentTrim(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  Jon Doe  ", "jon doe"},
		{"José García", "jose garcia"},
		{"JOHN DOE", "john doe"},
	}
	for _, tc := range cases {
		if got := NormalizeName(tc.in); got != tc.want {
			t.Errorf("NormalizeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeCompanyName_LegalSuffixStrip(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Acme Inc", "acme"},
		{"Acme GmbH", "acme"},
		{"Acme Ltd Co", "acme"},
		{"Acme Corp of America", "acme corp of america"}, // "america" isn't a suffix — no strip
	}
	for _, tc := range cases {
		if got := NormalizeCompanyName(tc.in); got != tc.want {
			t.Errorf("NormalizeCompanyName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPersonConfidence_GoldenExamples(t *testing.T) {
	nameSim := JaroWinkler(NormalizeName("jon doe"), NormalizeName("john doe"))
	sameOrg := PersonConfidence(nameSim, 1.0)
	if diff := sameOrg - 0.982; diff > 0.001 || diff < -0.001 {
		t.Errorf("same-org confidence = %v, want ~0.982", sameOrg)
	}
	if sameOrg < DedupeReviewThreshold {
		t.Errorf("same-org confidence %v must clear the %v review threshold (FUZZY_REVIEW)", sameOrg, DedupeReviewThreshold)
	}

	diffOrg := PersonConfidence(nameSim, 0.0)
	if diff := diffOrg - 0.532; diff > 0.001 || diff < -0.001 {
		t.Errorf("different-org confidence = %v, want ~0.532", diffOrg)
	}
	if diffOrg >= DedupeReviewThreshold {
		t.Errorf("different-org confidence %v must NOT clear the %v review threshold (NO_MATCH)", diffOrg, dedupeNameWeight*nameSim)
	}
}

func TestOrgNameSim_LegalSuffixNormalizedEqual(t *testing.T) {
	// PO-F-2: "Acme Inc" vs "Acme GmbH" both normalize to "acme" -> name_sim=1.0 -> FUZZY_REVIEW.
	sim := JaroWinkler(NormalizeCompanyName("Acme Inc"), NormalizeCompanyName("Acme GmbH"))
	if sim != 1.0 {
		t.Errorf("org name_sim = %v, want 1.0", sim)
	}
	if sim < DedupeReviewThreshold {
		t.Errorf("org name_sim %v must clear the review threshold", sim)
	}
}

func TestOrgMatchScore_Ladder(t *testing.T) {
	orgA := "org-a"
	orgB := "org-b"
	// 1.0: new person's domain-derived org == candidate's current-primary employment org.
	if got := OrgMatchScore(&orgA, &orgA, nil, "", ""); got != 1.0 {
		t.Errorf("current-primary match = %v, want 1.0", got)
	}
	// 0.8: new person's domain-derived org == candidate's own domain-derived org (no employment confirmation).
	if got := OrgMatchScore(&orgA, nil, &orgA, "", ""); got != 0.8 {
		t.Errorf("domain-org match = %v, want 0.8", got)
	}
	// 0.5: free-text company strings normalize-equal after legal-suffix strip.
	if got := OrgMatchScore(nil, nil, nil, "Acme Inc", "Acme GmbH"); got != 0.5 {
		t.Errorf("free-text company match = %v, want 0.5", got)
	}
	// 0.0: no signal at all.
	if got := OrgMatchScore(nil, nil, nil, "", ""); got != 0.0 {
		t.Errorf("no-signal match = %v, want 0.0", got)
	}
	// mismatched orgs -> 0.0, never a partial credit.
	if got := OrgMatchScore(&orgA, &orgB, &orgB, "", ""); got != 0.0 {
		t.Errorf("mismatched orgs = %v, want 0.0 (current-primary must exactly match, %s != %s)", got, orgA, orgB)
	}
}
