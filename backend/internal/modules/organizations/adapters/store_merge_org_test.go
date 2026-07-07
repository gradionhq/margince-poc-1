//go:build integration

// store_merge_org_test.go — ported from modules/directory/store_merge_org_test.go
// (package crmcore → package adapters_test; type refs updated to organizations/adapters
// and organizations/domain).
package adapters_test

import (
	"errors"
	"testing"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// PO-AC-18: relink domains/deals/relationships/activity links/partner, zero
// orphaned FKs, one audit tx, one organization.merged event.
func TestOrgMergeRelinksDomainsDealsAndPartner(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	orgStore := orgAdapters.NewOrgStore(db)
	loser := mkOrg(ctx, t, orgStore, ws, "Loser Co")
	target := mkOrg(ctx, t, orgStore, ws, "Target Co")

	db.Exec(`INSERT INTO organization_domain (workspace_id, organization_id, domain, is_primary) VALUES ($1,$2,'loserco.com',true)`, ws, loser.ID)
	dealID := mkDealForMergeTest(t, db, ws)
	db.Exec(`UPDATE deal SET organization_id=$1::uuid WHERE id=$2::uuid`, loser.ID, dealID)
	db.Exec(`INSERT INTO partner (workspace_id, organization_id, source, captured_by) VALUES ($1,$2,'api','human:t')`, ws, loser.ID)

	merged, err := orgStore.Merge(ctx, loser.ID, target.ID, ws)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if merged.ID != target.ID {
		t.Fatalf("merge returned %s, want target %s", merged.ID, target.ID)
	}

	var loserMergedInto string
	var archived bool
	db.QueryRow(`SELECT merged_into_id::text, archived_at IS NOT NULL FROM organization WHERE id=$1::uuid`, loser.ID).Scan(&loserMergedInto, &archived)
	if loserMergedInto != target.ID || !archived {
		t.Fatalf("loser org merged_into_id=%s archived=%v, want %s/true", loserMergedInto, archived, target.ID)
	}
	assertNoRows(t, db, `SELECT 1 FROM organization_domain WHERE organization_id=$1::uuid AND archived_at IS NULL`, loser.ID)

	var dealOrg string
	db.QueryRow(`SELECT organization_id::text FROM deal WHERE id=$1::uuid`, dealID).Scan(&dealOrg)
	if dealOrg != target.ID {
		t.Fatalf("deal.organization_id after merge = %s, want %s", dealOrg, target.ID)
	}
	var partnerOrg string
	db.QueryRow(`SELECT organization_id::text FROM partner WHERE organization_id=$1::uuid OR organization_id=$2::uuid ORDER BY (organization_id=$2::uuid) DESC LIMIT 1`, loser.ID, target.ID).Scan(&partnerOrg)
	if partnerOrg != target.ID {
		t.Fatalf("partner.organization_id after merge (target had none) = %s, want %s", partnerOrg, target.ID)
	}

	var auditCount, eventCount int
	db.QueryRow(`SELECT count(*) FROM audit_log WHERE action='merge' AND entity_type='organization' AND entity_id=$1::uuid`, loser.ID).Scan(&auditCount)
	db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='organization.merged' AND entity_id=$1::uuid`, loser.ID).Scan(&eventCount)
	if auditCount != 1 || eventCount != 1 {
		t.Fatalf("audit=%d event=%d, want 1/1", auditCount, eventCount)
	}
}

// PO-AC-M1 (org side): survivor's primary domain wins; loser's is demoted, not deleted.
func TestOrgMergePrimaryDomainConflictSurvivorWins(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	orgStore := orgAdapters.NewOrgStore(db)
	loser := mkOrg(ctx, t, orgStore, ws, "Loser Co")
	target := mkOrg(ctx, t, orgStore, ws, "Target Co")
	db.Exec(`INSERT INTO organization_domain (workspace_id, organization_id, domain, is_primary) VALUES ($1,$2,'loserdomain.com',true)`, ws, loser.ID)
	db.Exec(`INSERT INTO organization_domain (workspace_id, organization_id, domain, is_primary) VALUES ($1,$2,'targetdomain.com',true)`, ws, target.ID)

	if _, err := orgStore.Merge(ctx, loser.ID, target.ID, ws); err != nil {
		t.Fatalf("merge: %v", err)
	}
	var survivorPrimary string
	db.QueryRow(`SELECT domain FROM organization_domain WHERE organization_id=$1::uuid AND is_primary=true AND archived_at IS NULL`, target.ID).Scan(&survivorPrimary)
	if survivorPrimary != "targetdomain.com" {
		t.Fatalf("survivor primary domain = %q, want targetdomain.com", survivorPrimary)
	}
	var demoted int
	db.QueryRow(`SELECT count(*) FROM organization_domain WHERE organization_id=$1::uuid AND domain='loserdomain.com' AND is_primary=false`, target.ID).Scan(&demoted)
	if demoted != 1 {
		t.Fatalf("loser's conflicting primary domain must be demoted (not deleted), got count=%d", demoted)
	}
}

func TestOrgMergeSelfMergeRejected(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	orgStore := orgAdapters.NewOrgStore(db)
	o := mkOrg(ctx, t, orgStore, ws, "Solo Co")
	if _, err := orgStore.Merge(ctx, o.ID, o.ID, ws); !errors.Is(err, orgAdapters.ErrSelfMerge) {
		t.Fatalf("self-merge: want ErrSelfMerge, got %v", err)
	}
}

// PO-AC-18 completeness (org mirror of Task 2's FK-walk, shared helper in
// helpers_shared_test.go): every live FK into organization(id) is either
// relinked or a documented exception.
func TestOrgMergeFKWalkExhaustive(t *testing.T) {
	db := openTestDB(t)
	fks := fkIntoTable(t, db, "organization")
	relinked := map[string]bool{
		"organization_domain": true, "deal": true, "activity_link": true,
		"relationship": true, "partner": true,
	}
	leftAlone := map[string]bool{"organization": true /* parent_org_id + merged_into_id self-ref */}
	for table := range fks {
		if !relinked[table] && !leftAlone[table] {
			t.Fatalf("FK from %s into organization(id) is neither relinked nor documented as intentionally left — merge relink logic is incomplete for this table", table)
		}
	}
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	orgStore := orgAdapters.NewOrgStore(db)
	loser := mkOrg(ctx, t, orgStore, ws, "FKWalkLoserCo")
	target := mkOrg(ctx, t, orgStore, ws, "FKWalkTargetCo")
	db.Exec(`INSERT INTO organization_domain (workspace_id, organization_id, domain) VALUES ($1,$2,'fkwalkloser.com')`, ws, loser.ID)
	if _, err := orgStore.Merge(ctx, loser.ID, target.ID, ws); err != nil {
		t.Fatalf("merge: %v", err)
	}
	var domainCount int
	db.QueryRow(`SELECT count(*) FROM organization_domain WHERE organization_id=$1::uuid AND archived_at IS NULL`, loser.ID).Scan(&domainCount)
	if domainCount != 0 {
		t.Fatalf("organization_domain still has %d live row(s) pointing at the archived loser org — relink incomplete", domainCount)
	}
	var loserExists int
	db.QueryRow(`SELECT count(*) FROM organization WHERE id=$1::uuid`, loser.ID).Scan(&loserExists)
	if loserExists != 1 {
		t.Fatalf("loser org row must still exist post-merge (soft-archive, never delete) — got count=%d", loserExists)
	}
}
