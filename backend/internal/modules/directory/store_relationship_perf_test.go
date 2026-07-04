//go:build integration

package crmcore

import (
	"context"
	"strings"
	"testing"
	"time"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const wsRelPerf = "00000000-0000-0000-0000-0000000000d1"

// TestRelationshipList_OrgEmployment_P95AndExplain covers PO-AC-13: listing an
// organization's employment edges (idx_rel_org_people) must be index-served
// and p95 < 150ms over 50 rows.
func TestRelationshipList_OrgEmployment_P95AndExplain(t *testing.T) {
	db := openTestDB(t)
	setRLS(t, db, wsRelPerf)
	seedWorkspace(t, db, wsRelPerf)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelPerf})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	os := NewOrgStore(db)
	org, err := os.Create(ctx, Organization{WorkspaceID: wsRelPerf, DisplayName: "Perf Org " + uniq(), Classification: strPtr("prospect"), Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	ps := NewPersonStore(db)
	s := NewRelationshipStore(db)
	for i := 0; i < 50; i++ {
		p, err := ps.Create(ctx, Person{WorkspaceID: wsRelPerf, FullName: "Perf Person " + uniq(), Source: p0.Source, CapturedBy: p0.CapturedBy})
		if err != nil {
			t.Fatalf("seed person %d: %v", i, err)
		}
		if _, err := s.Create(ctx, Relationship{
			WorkspaceID: wsRelPerf, Kind: relKindEmployment, PersonID: &p.ID, OrganizationID: &org.ID,
			Source: p0.Source, CapturedBy: p0.CapturedBy,
		}); err != nil {
			t.Fatalf("seed employment %d: %v", i, err)
		}
	}

	t.Run("explain_no_seq_scan", func(t *testing.T) {
		explainSQL := `EXPLAIN SELECT id FROM relationship
			WHERE workspace_id='` + wsRelPerf + `'::uuid AND archived_at IS NULL
			  AND kind='employment' AND organization_id='` + org.ID + `'::uuid
			ORDER BY id LIMIT 51`
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal("begin tx:", err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
			t.Fatal("disable seqscan:", err)
		}
		rows, err := tx.QueryContext(ctx, explainSQL)
		if err != nil {
			t.Fatal("EXPLAIN:", err)
		}
		defer rows.Close()
		var plan strings.Builder
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal("scan plan line:", err)
			}
			plan.WriteString(line)
			plan.WriteString("\n")
		}
		if err := rows.Err(); err != nil {
			t.Fatal("plan rows:", err)
		}
		t.Logf("org+kind filter plan (seqscan=off):\n%s", plan.String())
		if strings.Contains(plan.String(), "Seq Scan on relationship") {
			t.Fatalf("organization_id+kind filter fell back to Seq Scan with seqscan off — index coverage required, plan:\n%s", plan.String())
		}
		wantIndexes := []string{"idx_rel_org_people", "idx_rel_person_orgs"}
		found := false
		for _, idx := range wantIndexes {
			if strings.Contains(plan.String(), idx) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected Index Scan on one of %v, got:\n%s", wantIndexes, plan.String())
		}
	})

	t.Run("p95_under_150ms", func(t *testing.T) {
		const iterations = 30
		durations := make([]time.Duration, 0, iterations)
		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, _, err := s.List(ctx, wsRelPerf, "", 50, RelationshipListFilter{Kind: relKindEmployment, OrganizationID: org.ID})
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("List iteration %d: %v", i, err)
			}
			durations = append(durations, elapsed)
		}
		p95 := percentile(durations, 95)
		t.Logf("organization_id+kind=employment p95 over %d iterations: %v", iterations, p95)
		if p95 > 150*time.Millisecond {
			t.Errorf("p95 %v exceeds 150ms budget", p95)
		}
	})
}

// TestRelationshipList_DealStakeholders_P95AndExplain covers DEAL-AC-10: the
// deal-side reverse lookup (idx_rel_deal_stakeholders) must be index-served
// and p95 < 150ms over 50 rows.
func TestRelationshipList_DealStakeholders_P95AndExplain(t *testing.T) {
	db := openTestDB(t)
	setRLS(t, db, wsRelPerf)
	seedWorkspace(t, db, wsRelPerf)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsRelPerf})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: wsRelPerf, Name: "RelPerf " + uniq()})
	if err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	sstore := deals.NewStageStore(db)
	st, err := sstore.Create(ctx, deals.Stage{WorkspaceID: wsRelPerf, PipelineID: pl.ID, Name: "S", Position: 1, Semantic: "open", WinProbability: 50})
	if err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	ds := NewDealStore(db)
	dSeed := NewDeal("RelPerf Deal "+uniq(), pl.ID, st.ID, p0)
	dSeed.WorkspaceID = wsRelPerf
	d, err := ds.Create(ctx, dSeed, "")
	if err != nil {
		t.Fatalf("seed deal: %v", err)
	}

	ps := NewPersonStore(db)
	s := NewRelationshipStore(db)
	roles := []string{"champion", "economic_buyer", "blocker", "influencer", "user"}
	for i := 0; i < 50; i++ {
		p, err := ps.Create(ctx, Person{WorkspaceID: wsRelPerf, FullName: "Stakeholder " + uniq(), Source: p0.Source, CapturedBy: p0.CapturedBy})
		if err != nil {
			t.Fatalf("seed person %d: %v", i, err)
		}
		// uq_rel_deal_person_role is keyed on (deal_id, person_id, role); a fresh
		// person per iteration means the role can safely cycle through the enum
		// without ever colliding on the same (deal, person) pair.
		if _, err := s.Create(ctx, Relationship{
			WorkspaceID: wsRelPerf, Kind: relKindDealStakeholder, DealID: &d.ID, PersonID: &p.ID,
			Role: strPtr(roles[i%len(roles)]), Source: p0.Source, CapturedBy: p0.CapturedBy,
		}); err != nil {
			t.Fatalf("seed stakeholder %d: %v", i, err)
		}
	}

	t.Run("explain_no_seq_scan", func(t *testing.T) {
		explainSQL := `EXPLAIN SELECT id FROM relationship
			WHERE workspace_id='` + wsRelPerf + `'::uuid AND archived_at IS NULL
			  AND kind='deal_stakeholder' AND deal_id='` + d.ID + `'::uuid
			ORDER BY id LIMIT 51`
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal("begin tx:", err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
			t.Fatal("disable seqscan:", err)
		}
		rows, err := tx.QueryContext(ctx, explainSQL)
		if err != nil {
			t.Fatal("EXPLAIN:", err)
		}
		defer rows.Close()
		var plan strings.Builder
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal("scan plan line:", err)
			}
			plan.WriteString(line)
			plan.WriteString("\n")
		}
		if err := rows.Err(); err != nil {
			t.Fatal("plan rows:", err)
		}
		t.Logf("deal_id+kind filter plan (seqscan=off):\n%s", plan.String())
		if strings.Contains(plan.String(), "Seq Scan on relationship") {
			t.Fatalf("deal_id+kind filter fell back to Seq Scan with seqscan off — index coverage required, plan:\n%s", plan.String())
		}
		wantIndexes := []string{"idx_rel_deal_stakeholders", "uq_rel_deal_person_role"}
		found := false
		for _, idx := range wantIndexes {
			if strings.Contains(plan.String(), idx) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected Index Scan on one of %v, got:\n%s", wantIndexes, plan.String())
		}
	})

	t.Run("p95_under_150ms", func(t *testing.T) {
		const iterations = 30
		durations := make([]time.Duration, 0, iterations)
		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, _, err := s.List(ctx, wsRelPerf, "", 50, RelationshipListFilter{Kind: relKindDealStakeholder, DealID: d.ID})
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("List iteration %d: %v", i, err)
			}
			durations = append(durations, elapsed)
		}
		p95 := percentile(durations, 95)
		t.Logf("deal_id+kind=deal_stakeholder p95 over %d iterations: %v", iterations, p95)
		if p95 > 150*time.Millisecond {
			t.Errorf("p95 %v exceeds 150ms budget", p95)
		}
	})
}
