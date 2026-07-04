//go:build integration

package crmcore

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func mergeTestCtx(ws string) context.Context {
	return crmctx.With(context.Background(), crmctx.Principal{UserID: "human:merge-test", TenantID: ws})
}

func mkPerson(ctx context.Context, t *testing.T, store *PersonStore, ws, name string) Person {
	t.Helper()
	p := NewPerson(name, prov.Provenance{Source: "api", CapturedBy: "human:merge-test"})
	p.WorkspaceID = ws
	created, err := store.Create(ctx, p, nil)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return created
}

func mkOrgForMergeTest(t *testing.T, db *sql.DB, ws string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO organization (workspace_id, name, classification, source, captured_by)
		VALUES ($1,'MergeTestOrg','prospect','api','human:t') RETURNING id`, ws).Scan(&id); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	return id
}

func mkDealForMergeTest(t *testing.T, db *sql.DB, ws string) string {
	t.Helper()
	var pipelineID, stageID, dealID string
	db.QueryRow(`INSERT INTO pipeline (workspace_id, name, is_default) VALUES ($1,'MergeTestPipeline',true) RETURNING id`, ws).Scan(&pipelineID)
	db.QueryRow(`INSERT INTO stage (workspace_id, pipeline_id, name, position) VALUES ($1,$2,'Open',1) RETURNING id`, ws, pipelineID).Scan(&stageID)
	if err := db.QueryRow(`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, source, captured_by)
		VALUES ($1,'MergeTestDeal',$2,$3,'api','human:t') RETURNING id`, ws, pipelineID, stageID).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	return dealID
}

// PO-AC-17: relink zero orphaned FKs, archive loser, one audit tx.
func TestPersonMergeRelinksEmailsPhonesRelationshipsAndActivityLinks(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	store := NewPersonStore(db)

	loser := mkPerson(ctx, t, store, ws, "Loser")
	target := mkPerson(ctx, t, store, ws, "Target")

	if _, err := db.Exec(`INSERT INTO person_email (workspace_id, person_id, email, is_primary, source, captured_by)
		VALUES ($1,$2,'loser@acme.com',true,'api','human:t')`, ws, loser.ID); err != nil {
		t.Fatalf("seed person_email: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO person_phone (workspace_id, person_id, phone, is_primary, source, captured_by)
		VALUES ($1,$2,'+15550000000',true,'api','human:t')`, ws, loser.ID); err != nil {
		t.Fatalf("seed person_phone: %v", err)
	}
	orgID := mkOrgForMergeTest(t, db, ws)
	if _, err := db.Exec(`INSERT INTO relationship (workspace_id, kind, person_id, organization_id, is_primary, source, captured_by)
		VALUES ($1,'employment',$2,$3,true,'api','human:t')`, ws, loser.ID, orgID); err != nil {
		t.Fatalf("seed relationship: %v", err)
	}

	merged, err := store.Merge(ctx, loser.ID, target.ID, ws)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if merged.ID != target.ID {
		t.Fatalf("merge returned %s, want target %s", merged.ID, target.ID)
	}

	var loserMergedInto string
	var archived bool
	if err := db.QueryRow(`SELECT merged_into_id::text, archived_at IS NOT NULL FROM person WHERE id=$1::uuid`, loser.ID).
		Scan(&loserMergedInto, &archived); err != nil {
		t.Fatalf("select loser: %v", err)
	}
	if loserMergedInto != target.ID || !archived {
		t.Fatalf("loser merged_into_id=%s archived=%v, want %s/true", loserMergedInto, archived, target.ID)
	}

	assertNoRows(t, db, `SELECT 1 FROM person_email WHERE person_id=$1::uuid`, loser.ID)
	assertNoRows(t, db, `SELECT 1 FROM person_phone WHERE person_id=$1::uuid`, loser.ID)
	assertNoRows(t, db, `SELECT 1 FROM relationship WHERE person_id=$1::uuid AND archived_at IS NULL`, loser.ID)

	var emailCount, phoneCount, relCount int
	db.QueryRow(`SELECT count(*) FROM person_email WHERE person_id=$1::uuid`, target.ID).Scan(&emailCount)
	db.QueryRow(`SELECT count(*) FROM person_phone WHERE person_id=$1::uuid`, target.ID).Scan(&phoneCount)
	db.QueryRow(`SELECT count(*) FROM relationship WHERE person_id=$1::uuid AND archived_at IS NULL`, target.ID).Scan(&relCount)
	if emailCount != 1 || phoneCount != 1 || relCount != 1 {
		t.Fatalf("target counts email=%d phone=%d rel=%d, want 1/1/1", emailCount, phoneCount, relCount)
	}

	var auditCount int
	db.QueryRow(`SELECT count(*) FROM audit_log WHERE action='merge' AND entity_type='person' AND entity_id=$1::uuid`, loser.ID).Scan(&auditCount)
	if auditCount != 1 {
		t.Fatalf("audit_log merge rows for loser = %d, want 1", auditCount)
	}
	var beforeRaw []byte
	db.QueryRow(`SELECT before FROM audit_log WHERE action='merge' AND entity_type='person' AND entity_id=$1::uuid`, loser.ID).Scan(&beforeRaw)
	if len(beforeRaw) == 0 || string(beforeRaw) == "null" {
		t.Fatalf("audit before-snapshot is empty — merge must be reversible from the audit row")
	}

	var eventCount int
	db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='person.merged' AND entity_id=$1::uuid`, loser.ID).Scan(&eventCount)
	if eventCount != 1 {
		t.Fatalf("person.merged events for loser = %d, want exactly 1", eventCount)
	}
	var updatedEvents int
	db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic IN ('person.updated','person.archived') AND entity_id=$1::uuid`, loser.ID).Scan(&updatedEvents)
	if updatedEvents != 0 {
		t.Fatalf("merge must not also emit a generic updated/archived event (EVT-SEM-2), got %d", updatedEvents)
	}
}

// PO-AC-M1: survivor's primary phone wins; loser's conflicting primary demoted, not deleted.
func TestPersonMergePrimaryPhoneConflictSurvivorWins(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	store := NewPersonStore(db)
	loser := mkPerson(ctx, t, store, ws, "Loser")
	target := mkPerson(ctx, t, store, ws, "Target")

	db.Exec(`INSERT INTO person_phone (workspace_id, person_id, phone, phone_type, is_primary, source, captured_by)
		VALUES ($1,$2,'+15551111111','work',true,'api','human:t')`, ws, loser.ID)
	db.Exec(`INSERT INTO person_phone (workspace_id, person_id, phone, phone_type, is_primary, source, captured_by)
		VALUES ($1,$2,'+15552222222','work',true,'api','human:t')`, ws, target.ID)

	if _, err := store.Merge(ctx, loser.ID, target.ID, ws); err != nil {
		t.Fatalf("merge: %v", err)
	}
	var survivorPrimary string
	db.QueryRow(`SELECT phone FROM person_phone WHERE person_id=$1::uuid AND phone_type='work' AND is_primary=true`, target.ID).Scan(&survivorPrimary)
	if survivorPrimary != "+15552222222" {
		t.Fatalf("survivor primary phone = %q, want target's original +15552222222", survivorPrimary)
	}
	var demotedCount int
	db.QueryRow(`SELECT count(*) FROM person_phone WHERE person_id=$1::uuid AND phone='+15551111111' AND is_primary=false`, target.ID).Scan(&demotedCount)
	if demotedCount != 1 {
		t.Fatalf("loser's conflicting primary phone must be demoted (not deleted) on relink, got count=%d", demotedCount)
	}
}

// PO-AC-M1: survivor's primary email wins; loser's conflicting primary demoted, not deleted.
func TestPersonMergePrimaryEmailConflictSurvivorWins(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	store := NewPersonStore(db)
	loser := mkPerson(ctx, t, store, ws, "Loser")
	target := mkPerson(ctx, t, store, ws, "Target")

	db.Exec(`INSERT INTO person_email (workspace_id, person_id, email, is_primary, source, captured_by)
		VALUES ($1,$2,'loser-primary@acme.com',true,'api','human:t')`, ws, loser.ID)
	db.Exec(`INSERT INTO person_email (workspace_id, person_id, email, is_primary, source, captured_by)
		VALUES ($1,$2,'target-primary@acme.com',true,'api','human:t')`, ws, target.ID)

	if _, err := store.Merge(ctx, loser.ID, target.ID, ws); err != nil {
		t.Fatalf("merge: %v", err)
	}
	var survivorPrimary string
	db.QueryRow(`SELECT email FROM person_email WHERE person_id=$1::uuid AND is_primary=true AND archived_at IS NULL`, target.ID).Scan(&survivorPrimary)
	if survivorPrimary != "target-primary@acme.com" {
		t.Fatalf("survivor primary email = %q, want target's original target-primary@acme.com", survivorPrimary)
	}
	var demotedCount int
	db.QueryRow(`SELECT count(*) FROM person_email WHERE person_id=$1::uuid AND email='loser-primary@acme.com' AND is_primary=false`, target.ID).Scan(&demotedCount)
	if demotedCount != 1 {
		t.Fatalf("loser's conflicting primary email must be demoted (not deleted) on relink, got count=%d", demotedCount)
	}
}

// PO-AC-M1: survivor's current-primary employer relationship wins; loser's conflicting
// primary employment row is demoted (is_primary=false), not deleted or archived.
func TestPersonMergeCurrentPrimaryEmployerConflictSurvivorWins(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	store := NewPersonStore(db)
	loser := mkPerson(ctx, t, store, ws, "Loser")
	target := mkPerson(ctx, t, store, ws, "Target")
	loserOrg := mkOrgForMergeTest(t, db, ws)
	targetOrg := mkOrgForMergeTest(t, db, ws)

	db.Exec(`INSERT INTO relationship (workspace_id, kind, person_id, organization_id, is_primary, source, captured_by)
		VALUES ($1,'employment',$2,$3,true,'api','human:t')`, ws, loser.ID, loserOrg)
	db.Exec(`INSERT INTO relationship (workspace_id, kind, person_id, organization_id, is_primary, source, captured_by)
		VALUES ($1,'employment',$2,$3,true,'api','human:t')`, ws, target.ID, targetOrg)

	if _, err := store.Merge(ctx, loser.ID, target.ID, ws); err != nil {
		t.Fatalf("merge: %v", err)
	}
	var survivorPrimaryOrg string
	db.QueryRow(`SELECT organization_id::text FROM relationship WHERE person_id=$1::uuid AND kind='employment' AND is_primary=true AND ended_at IS NULL AND archived_at IS NULL`, target.ID).Scan(&survivorPrimaryOrg)
	if survivorPrimaryOrg != targetOrg {
		t.Fatalf("survivor current-primary employer = %s, want target's original %s", survivorPrimaryOrg, targetOrg)
	}
	var demotedCount int
	db.QueryRow(`SELECT count(*) FROM relationship WHERE person_id=$1::uuid AND organization_id=$2::uuid AND kind='employment' AND is_primary=false AND archived_at IS NULL`, target.ID, loserOrg).Scan(&demotedCount)
	if demotedCount != 1 {
		t.Fatalf("loser's conflicting current-primary employment row must be demoted (not deleted/archived) on relink, got count=%d", demotedCount)
	}
}

// PO-AC-M2: duplicate deal-stakeholder rows collapse instead of violating uq_rel_deal_person_role.
func TestPersonMergeStakeholderDuplicateCollapses(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	store := NewPersonStore(db)
	loser := mkPerson(ctx, t, store, ws, "Loser")
	target := mkPerson(ctx, t, store, ws, "Target")
	dealID := mkDealForMergeTest(t, db, ws)

	db.Exec(`INSERT INTO relationship (workspace_id, kind, person_id, deal_id, role, source, captured_by)
		VALUES ($1,'deal_stakeholder',$2,$3,'champion','api','human:t')`, ws, loser.ID, dealID)
	db.Exec(`INSERT INTO relationship (workspace_id, kind, person_id, deal_id, role, source, captured_by)
		VALUES ($1,'deal_stakeholder',$2,$3,'champion','api','human:t')`, ws, target.ID, dealID)

	if _, err := store.Merge(ctx, loser.ID, target.ID, ws); err != nil {
		t.Fatalf("merge must not violate uq_rel_deal_person_role: %v", err)
	}
	var liveCount int
	db.QueryRow(`SELECT count(*) FROM relationship WHERE deal_id=$1::uuid AND role='champion' AND kind='deal_stakeholder' AND archived_at IS NULL`, dealID).Scan(&liveCount)
	if liveCount != 1 {
		t.Fatalf("live champion stakeholder rows for deal = %d, want 1 (collapsed)", liveCount)
	}
}

// PO-AC-M3: self-merge is rejected.
func TestPersonMergeSelfMergeRejected(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	store := NewPersonStore(db)
	p := mkPerson(ctx, t, store, ws, "Solo")
	if _, err := store.Merge(ctx, p.ID, p.ID, ws); !errors.Is(err, ErrSelfMerge) {
		t.Fatalf("self-merge: want ErrSelfMerge, got %v", err)
	}
}

// PO-AC-M4: merging an already-merged loser (or into an already-merged target) 422s
// with a pointer to the actual survivor, following the chain.
func TestPersonMergeAlreadyMergedFollowsChain(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	store := NewPersonStore(db)
	a := mkPerson(ctx, t, store, ws, "A")
	b := mkPerson(ctx, t, store, ws, "B")
	c := mkPerson(ctx, t, store, ws, "C")

	if _, err := store.Merge(ctx, a.ID, b.ID, ws); err != nil {
		t.Fatalf("first merge a->b: %v", err)
	}
	// Re-merging A (now pointing at B) must 422, pointer = B.
	_, err := store.Merge(ctx, a.ID, c.ID, ws)
	var already *ErrAlreadyMerged
	if !errors.As(err, &already) || already.SurvivorID != b.ID {
		t.Fatalf("re-merge of already-merged A: want ErrAlreadyMerged{SurvivorID:%s}, got %v", b.ID, err)
	}
	// Merging some other person INTO A (the now-archived loser) must also reject,
	// pointing at B — the target itself is already merged elsewhere.
	d := mkPerson(ctx, t, store, ws, "D")
	_, err = store.Merge(ctx, d.ID, a.ID, ws)
	var targetInvalid *ErrMergeTargetInvalid
	if !errors.As(err, &targetInvalid) || targetInvalid.SurvivorID != b.ID {
		t.Fatalf("merge into already-merged target: want ErrMergeTargetInvalid{SurvivorID:%s}, got %v", b.ID, err)
	}
}

// PO-AC-M5: two concurrent merges of the same loser — the loser wins the row-lock race,
// the other gets ErrVersionSkew. Deterministic under Postgres READ COMMITTED: the
// blocked UPDATE re-evaluates its WHERE version=$readVersion clause against the
// already-committed row once unblocked and finds no match.
func TestPersonMergeConcurrentLoses409VersionSkew(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := mergeTestCtx(ws)
	store := NewPersonStore(db)
	loser := mkPerson(ctx, t, store, ws, "Loser")
	targetB := mkPerson(ctx, t, store, ws, "TargetB")
	targetC := mkPerson(ctx, t, store, ws, "TargetC")

	type result struct {
		err error
	}
	results := make(chan result, 2)
	go func() { _, err := store.Merge(ctx, loser.ID, targetB.ID, ws); results <- result{err} }()
	go func() { _, err := store.Merge(ctx, loser.ID, targetC.ID, ws); results <- result{err} }()

	r1, r2 := <-results, <-results
	successes, skews := 0, 0
	for _, r := range []result{r1, r2} {
		switch {
		case r.err == nil:
			successes++
		case errors.Is(r.err, errs.ErrVersionSkew):
			skews++
		default:
			t.Fatalf("unexpected concurrent-merge error: %v", r.err)
		}
	}
	if successes != 1 || skews != 1 {
		t.Fatalf("concurrent merges: got %d successes / %d version_skew, want exactly 1/1", successes, skews)
	}
}
