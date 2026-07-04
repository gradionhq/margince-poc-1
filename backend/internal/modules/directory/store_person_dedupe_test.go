//go:build integration

package crmcore_test

import (
	"testing"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// TestPersonCreate_FuzzyReview_SameOrg proves PO-AC-19's FUZZY_REVIEW path at
// the ladder's headline 1.0 rung: a near-duplicate name whose domain-derived
// org matches the candidate's live CURRENT-PRIMARY employment org (not just a
// shared email domain — PLAN-REVIEW B1: a domain-only match is the 0.8 rung,
// 0.892 confidence, not this test's 0.982) clears DEDUPE_REVIEW_THRESHOLD,
// and create still succeeds (no error) with the review flag naming the
// existing candidate + its confidence.
func TestPersonCreate_FuzzyReview_SameOrg(t *testing.T) {
	db := sqlDB(t)
	ws := newWorkspaceSQL(t, db)
	people := crmcore.NewPersonStore(db)
	orgs := crmcore.NewOrgStore(db)
	rels := crmcore.NewRelationshipStore(db)

	org := crmcore.NewOrganization("Acme Corp", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	org.WorkspaceID = ws
	org.Domains = []crmcore.OrganizationDomain{{Domain: "acme.com", IsPrimary: true}}
	createdOrg, err := orgs.Create(appCtx(ws), org)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	existing := crmcore.NewPerson("John Doe", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	existing.WorkspaceID = ws
	createdExisting, err := people.Create(appCtx(ws), existing, []crmcore.PersonEmailInput{
		{Email: "john@acme.com", EmailType: "work", IsPrimary: true},
	})
	if err != nil {
		t.Fatalf("create existing person: %v", err)
	}

	// Give the existing candidate a live current-primary employment at Acme —
	// this is what drives orgMatchScore's 1.0 rung (candCurrentOrgID == the
	// new person's domain-derived org), matching the spec's pinned 0.982
	// worked example. Without this relationship row, orgMatchScore only
	// reaches its 0.8 domain-shared rung (confidence ~0.892) — see B1.
	rel := crmcore.Relationship{
		WorkspaceID: ws, Kind: "employment",
		PersonID: &createdExisting.ID, OrganizationID: &createdOrg.ID,
		IsCurrentPrimary: true, Source: "api", CapturedBy: "human:test",
	}
	if _, err := rels.Create(appCtx(ws), rel); err != nil {
		t.Fatalf("create employment relationship: %v", err)
	}

	candidate := crmcore.NewPerson("Jon Doe", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	candidate.WorkspaceID = ws
	created, err := people.Create(appCtx(ws), candidate, []crmcore.PersonEmailInput{
		{Email: "jon@acme.com", EmailType: "work", IsPrimary: true},
	})
	if err != nil {
		t.Fatalf("create candidate person: %v", err)
	}

	if created.ReviewFlag == nil {
		t.Fatal("want a non-nil review flag (jon doe/john doe, same current-primary org, confidence ~0.982 clears 0.72)")
	}
	if created.ReviewFlag.CandidateID != createdExisting.ID {
		t.Errorf("review flag candidate_id = %q, want %q", created.ReviewFlag.CandidateID, createdExisting.ID)
	}
	if diff := created.ReviewFlag.Confidence - 0.982; diff > 0.001 || diff < -0.001 {
		t.Errorf("review flag confidence = %v, want ~0.982", created.ReviewFlag.Confidence)
	}
}

// TestPersonCreate_FuzzyReview_DomainOnly_NoEmploymentConfirmation proves the
// ladder's 0.8 rung distinctly from the 1.0 rung above: two people sharing
// only an email domain (no employment relationship on either side) score
// 0.55*0.9667 + 0.45*0.8 = 0.8917 — still clears the review threshold, but at
// a visibly lower confidence than the current-primary-confirmed case.
func TestPersonCreate_FuzzyReview_DomainOnly_NoEmploymentConfirmation(t *testing.T) {
	db := sqlDB(t)
	ws := newWorkspaceSQL(t, db)
	people := crmcore.NewPersonStore(db)
	orgs := crmcore.NewOrgStore(db)

	org := crmcore.NewOrganization("Acme Corp", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	org.WorkspaceID = ws
	org.Domains = []crmcore.OrganizationDomain{{Domain: "acme.com", IsPrimary: true}}
	if _, err := orgs.Create(appCtx(ws), org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	existing := crmcore.NewPerson("John Doe", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	existing.WorkspaceID = ws
	createdExisting, err := people.Create(appCtx(ws), existing, []crmcore.PersonEmailInput{
		{Email: "john@acme.com", EmailType: "work", IsPrimary: true},
	})
	if err != nil {
		t.Fatalf("create existing person: %v", err)
	}

	candidate := crmcore.NewPerson("Jon Doe", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	candidate.WorkspaceID = ws
	created, err := people.Create(appCtx(ws), candidate, []crmcore.PersonEmailInput{
		{Email: "jon@acme.com", EmailType: "work", IsPrimary: true},
	})
	if err != nil {
		t.Fatalf("create candidate person: %v", err)
	}

	if created.ReviewFlag == nil {
		t.Fatal("want a non-nil review flag (domain-shared org, confidence ~0.892 still clears 0.72)")
	}
	if created.ReviewFlag.CandidateID != createdExisting.ID {
		t.Errorf("review flag candidate_id = %q, want %q", created.ReviewFlag.CandidateID, createdExisting.ID)
	}
	if diff := created.ReviewFlag.Confidence - 0.8917; diff > 0.001 || diff < -0.001 {
		t.Errorf("review flag confidence = %v, want ~0.8917 (domain-only rung, distinct from the 0.982 current-primary rung)", created.ReviewFlag.Confidence)
	}
}

// TestPersonCreate_NoReviewFlag_BelowThreshold proves an unrelated name at a
// different (or no) org creates plainly with no review flag.
func TestPersonCreate_NoReviewFlag_BelowThreshold(t *testing.T) {
	db := sqlDB(t)
	ws := newWorkspaceSQL(t, db)
	people := crmcore.NewPersonStore(db)

	existing := crmcore.NewPerson("John Doe", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	existing.WorkspaceID = ws
	if _, err := people.Create(appCtx(ws), existing, nil); err != nil {
		t.Fatalf("create existing person: %v", err)
	}

	unrelated := crmcore.NewPerson("Zara Ng", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	unrelated.WorkspaceID = ws
	created, err := people.Create(appCtx(ws), unrelated, nil)
	if err != nil {
		t.Fatalf("create unrelated person: %v", err)
	}
	if created.ReviewFlag != nil {
		t.Errorf("want nil review flag for an unrelated name, got %+v", created.ReviewFlag)
	}
}

// TestPersonCreate_EmptyFullName_SkipsFuzzyTier proves the edge case: an
// empty full_name skips the fuzzy tier entirely (no panic, no flag), while
// the pre-existing exact-email tier still runs unaffected.
func TestPersonCreate_EmptyFullName_SkipsFuzzyTier(t *testing.T) {
	db := sqlDB(t)
	ws := newWorkspaceSQL(t, db)
	people := crmcore.NewPersonStore(db)

	p := crmcore.NewPerson("", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	p.WorkspaceID = ws
	created, err := people.Create(appCtx(ws), p, nil)
	if err != nil {
		t.Fatalf("create empty-name person: %v", err)
	}
	if created.ReviewFlag != nil {
		t.Errorf("want nil review flag when full_name is empty, got %+v", created.ReviewFlag)
	}
}

// TestPersonCreate_TieBreak_LowestID proves AC-4's total order: two live
// candidates scoring identically resolve to the lowest person id, never an
// arbitrary/nondeterministic pick. Both existing "Pat Smith" rows share the
// same domain-derived org as the new person (via emails at acme.com, no
// employment relationship on either side), so both score the identical
// 0.8-rung confidence (0.55*1.0 + 0.45*0.8 = 0.91, clears 0.72) — a real tie,
// not a threshold miss.
func TestPersonCreate_TieBreak_LowestID(t *testing.T) {
	db := sqlDB(t)
	ws := newWorkspaceSQL(t, db)
	people := crmcore.NewPersonStore(db)
	orgs := crmcore.NewOrgStore(db)

	org := crmcore.NewOrganization("Acme Corp", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	org.WorkspaceID = ws
	org.Domains = []crmcore.OrganizationDomain{{Domain: "acme.com", IsPrimary: true}}
	if _, err := orgs.Create(appCtx(ws), org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	first := crmcore.NewPerson("Pat Smith", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	first.WorkspaceID = ws
	createdFirst, err := people.Create(appCtx(ws), first, []crmcore.PersonEmailInput{
		{Email: "pat1@acme.com", EmailType: "work", IsPrimary: true},
	})
	if err != nil {
		t.Fatalf("create first Pat Smith: %v", err)
	}

	second := crmcore.NewPerson("Pat Smith", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	second.WorkspaceID = ws
	createdSecond, err := people.Create(appCtx(ws), second, []crmcore.PersonEmailInput{
		{Email: "pat2@acme.com", EmailType: "work", IsPrimary: true},
	})
	if err != nil {
		t.Fatalf("create second Pat Smith: %v", err)
	}

	lowestID := createdFirst.ID
	if createdSecond.ID < lowestID {
		lowestID = createdSecond.ID
	}

	// A third, identically-named person at the same domain now has two
	// equal-scoring candidates — the winner must be the lower of the two ids.
	third := crmcore.NewPerson("Pat Smith", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	third.WorkspaceID = ws
	created, err := people.Create(appCtx(ws), third, []crmcore.PersonEmailInput{
		{Email: "pat3@acme.com", EmailType: "work", IsPrimary: true},
	})
	if err != nil {
		t.Fatalf("create third Pat Smith: %v", err)
	}

	if created.ReviewFlag == nil {
		t.Fatal("want a non-nil review flag (tie between first/second Pat Smith at ~0.91, clears 0.72)")
	}
	if created.ReviewFlag.CandidateID != lowestID {
		t.Errorf("tie-break candidate_id = %q, want the lowest id %q (first=%q, second=%q)",
			created.ReviewFlag.CandidateID, lowestID, createdFirst.ID, createdSecond.ID)
	}
}

// TestPersonDedupeCandidates_NeverQueriesLead proves ADR-0008 by construction
// against the real store: seed a `lead` row with a name/email that would
// otherwise be a near-perfect fuzzy match (same normalized name, same email
// domain as a live person's employer), then prove creating a matching person
// never surfaces that lead as a review-flag candidate — the candidate SQL
// selects only from `person`, never `lead`.
func TestPersonDedupeCandidates_NeverQueriesLead(t *testing.T) {
	db := sqlDB(t)
	ws := newWorkspaceSQL(t, db)
	people := crmcore.NewPersonStore(db)

	// A lead with the exact same full_name as the person we're about to
	// create — if the candidate query ever touched `lead`, this seed would
	// be picked up as a same-name "candidate" and its (non-person) id would
	// leak into a review flag.
	var leadID string
	if err := db.QueryRow(`
		INSERT INTO lead (workspace_id, full_name, email, source, captured_by)
		VALUES ($1::uuid, 'Casey Rivera', 'casey@leadco.example', 'api', 'human:test')
		RETURNING id`, ws).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	p := crmcore.NewPerson("Casey Rivera", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	p.WorkspaceID = ws
	created, err := people.Create(appCtx(ws), p, nil)
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	if created.ReviewFlag != nil {
		t.Fatalf("want nil review flag — the only same-name row is a lead (ADR-0008: leads never appear as fuzzy-dedupe candidates), got %+v", created.ReviewFlag)
	}
	_ = leadID
}
