//go:build integration

// store_org_dedupe_test.go — ported from modules/directory/store_org_dedupe_test.go
// (package crmcore_test → package adapters_test; imports updated to
// organizations/adapters and organizations/domain).
package adapters_test

import (
	"testing"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgDomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// TestOrgCreate_FuzzyReview_LegalSuffixNormalizedEqual proves PO-F-2: "Acme
// Inc" vs an existing "Acme GmbH" both normalize to "acme" -> name_sim=1.0,
// clears the review threshold, create still succeeds with the flag.
func TestOrgCreate_FuzzyReview_LegalSuffixNormalizedEqual(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	orgs := orgAdapters.NewOrgStore(db)

	existing := orgDomain.NewOrganization("Acme GmbH", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	existing.WorkspaceID = ws
	createdExisting, err := orgs.Create(pgtest.AppCtx(ws), existing, nil)
	if err != nil {
		t.Fatalf("create existing org: %v", err)
	}

	candidate := orgDomain.NewOrganization("Acme Inc", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	candidate.WorkspaceID = ws
	created, err := orgs.Create(pgtest.AppCtx(ws), candidate, nil)
	if err != nil {
		t.Fatalf("create candidate org: %v", err)
	}

	if created.ReviewFlag == nil {
		t.Fatal("want a non-nil review flag (Acme Inc/Acme GmbH normalize-equal, name_sim=1.0)")
	}
	if created.ReviewFlag.CandidateID != createdExisting.ID {
		t.Errorf("review flag candidate_id = %q, want %q", created.ReviewFlag.CandidateID, createdExisting.ID)
	}
	if created.ReviewFlag.Confidence != 1.0 {
		t.Errorf("review flag confidence = %v, want 1.0", created.ReviewFlag.Confidence)
	}
}

// TestOrgCreate_NoReviewFlag_UnrelatedName proves an unrelated org name
// creates plainly with no review flag.
func TestOrgCreate_NoReviewFlag_UnrelatedName(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	orgs := orgAdapters.NewOrgStore(db)

	existing := orgDomain.NewOrganization("Acme GmbH", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	existing.WorkspaceID = ws
	if _, err := orgs.Create(pgtest.AppCtx(ws), existing, nil); err != nil {
		t.Fatalf("create existing org: %v", err)
	}

	unrelated := orgDomain.NewOrganization("Zephyr Robotics", prov.Provenance{Source: "api", CapturedBy: "human:test"})
	unrelated.WorkspaceID = ws
	created, err := orgs.Create(pgtest.AppCtx(ws), unrelated, nil)
	if err != nil {
		t.Fatalf("create unrelated org: %v", err)
	}
	if created.ReviewFlag != nil {
		t.Errorf("want nil review flag for an unrelated org name, got %+v", created.ReviewFlag)
	}
}
