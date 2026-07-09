//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/activities"
	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	"github.com/gradionhq/margince/backend/internal/modules/records/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

type recordingActivityCreator struct {
	activity actdomain.Activity
	called   int
}

func (r *recordingActivityCreator) Create(_ context.Context, a actdomain.Activity) (actdomain.Activity, bool, error) {
	r.called++
	r.activity = a
	return a, true, nil
}

// seedDealForAudit seeds the minimum deal hierarchy (pipeline + stage + deal) and returns the dealID.
func seedDealForAudit(ctx context.Context, t *testing.T, db *sql.DB, ws string) string {
	t.Helper()
	var pipelineID, stageID, dealID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO pipeline (workspace_id, name) VALUES ($1,'Audit Pipeline') RETURNING id`, ws).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO stage (workspace_id, pipeline_id, name, position, semantic) VALUES ($1,$2,'Open',0,'open') RETURNING id`,
		ws, pipelineID).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, source, captured_by, version)
		 VALUES ($1,'Audit Deal',$2,$3,'test','human:audit-test',1) RETURNING id`,
		ws, pipelineID, stageID).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	return dealID
}

func TestWriteDownloadAudit_DealBound_AppearsOnTimeline(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:audit-test", TenantID: ws})

	dealID := seedDealForAudit(ctx, t, db, ws)
	actStore := activities.NewActivityStore(db)

	if err := adapters.WriteDownloadAudit(ctx, actStore, ws, domain.EntityTypeDeal, dealID, "report.pdf"); err != nil {
		t.Fatalf("WriteDownloadAudit: %v", err)
	}

	// Verify the activity row was written and appears on the deal's timeline (RD-AC-9).
	items, _, err := actStore.List(ctx, ws, domain.EntityTypeDeal, dealID, "", 20)
	if err != nil {
		t.Fatalf("List activities: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 timeline activity for deal, got %d", len(items))
	}
	a := items[0]
	if a.Kind != "note" {
		t.Errorf("expected Kind=note, got %q", a.Kind)
	}
	if a.Subject == nil || !strings.Contains(*a.Subject, "report.pdf") {
		t.Errorf("expected Subject to contain filename, got %v", a.Subject)
	}
	if a.Source != "system" {
		t.Errorf("expected Source=system, got %q", a.Source)
	}
	if a.CapturedBy != "system:attachment-download-audit" {
		t.Errorf("expected CapturedBy=system:attachment-download-audit, got %q", a.CapturedBy)
	}

	// Also verify exactly one activity_link row was written for this deal.
	var linkCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM activity_link WHERE activity_id=$1 AND deal_id=$2::uuid`,
		a.ID, dealID).Scan(&linkCount); err != nil {
		t.Fatalf("count activity_link: %v", err)
	}
	if linkCount != 1 {
		t.Fatalf("expected exactly 1 activity_link row for deal, got %d", linkCount)
	}
}

func TestWriteDownloadAudit_LeadBound_AuditedButNotLinked(t *testing.T) {
	// For lead/activity bindings, WriteDownloadAudit writes an Activity row but
	// no activity_link row — activity_link's own DB CHECK cannot bind lead/activity
	// entity types. This is the accepted, documented gap (Constraint 5). The activity
	// is still auditable via GET /activities/{id} but does not appear on a timeline.
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:audit-test", TenantID: ws})

	var leadID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO lead (workspace_id, full_name, source, captured_by, version)
		 VALUES ($1,'Audit Lead','test','human:audit-test',1) RETURNING id`,
		ws).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	actStore := activities.NewActivityStore(db)

	if err := adapters.WriteDownloadAudit(ctx, actStore, ws, domain.EntityTypeLead, leadID, "lead-doc.pdf"); err != nil {
		t.Fatalf("WriteDownloadAudit for lead: %v", err)
	}

	// Verify an activity row was written (audited).
	var actCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM activity WHERE workspace_id=$1 AND source='system' AND captured_by='system:attachment-download-audit'`,
		ws).Scan(&actCount); err != nil {
		t.Fatalf("count activities: %v", err)
	}
	if actCount != 1 {
		t.Fatalf("expected exactly 1 audit activity row for lead, got %d", actCount)
	}

	// Verify NO activity_link row was written — activity_link CHECK forbids lead bindings.
	var linkCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM activity_link al
		 JOIN activity a ON a.id = al.activity_id
		 WHERE a.workspace_id=$1 AND a.captured_by='system:attachment-download-audit'`,
		ws).Scan(&linkCount); err != nil {
		t.Fatalf("count activity_link: %v", err)
	}
	if linkCount != 0 {
		t.Fatalf("expected 0 activity_link rows for lead-bound download audit, got %d (documented gap: activity_link CHECK forbids lead)", linkCount)
	}
}

func TestWriteRequestAccessAudit_WritesLinkedActivity(t *testing.T) {
	// UserID is bare, matching production (crmctx.Principal.UserID never
	// carries a pre-baked "human:" prefix — see auth_handler.go's HandleLogin,
	// which casts the id straight into a Postgres $1::uuid). requestCapturedBy
	// must add the prefix itself.
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "audit-test", TenantID: "ws-1"})
	store := &recordingActivityCreator{}

	if err := adapters.WriteRequestAccessAudit(ctx, store, "ws-1", domain.EntityTypeDeal, "deal-1", "report.pdf"); err != nil {
		t.Fatalf("WriteRequestAccessAudit: %v", err)
	}
	if store.called != 1 {
		t.Fatalf("expected 1 activity create, got %d", store.called)
	}
	if store.activity.CapturedBy != "human:audit-test" {
		t.Fatalf("expected captured_by to follow the request principal, got %q", store.activity.CapturedBy)
	}
	if store.activity.Subject == nil || !strings.Contains(*store.activity.Subject, "Access requested: report.pdf") {
		t.Fatalf("expected access-request subject, got %v", store.activity.Subject)
	}
	if len(store.activity.Links) != 1 || store.activity.Links[0].EntityType != domain.EntityTypeDeal || store.activity.Links[0].EntityID != "deal-1" {
		t.Fatalf("expected deal link, got %+v", store.activity.Links)
	}
}

func TestWriteExtractionAcceptAudit_WritesSourceQuoteAndCapturedBy(t *testing.T) {
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:audit-test", TenantID: "ws-1"})
	store := &recordingActivityCreator{}

	if err := adapters.WriteExtractionAcceptAudit(ctx, store, "ws-1", domain.EntityTypeDeal, "deal-1", "name", "Acme Corp", "human:audit-test"); err != nil {
		t.Fatalf("WriteExtractionAcceptAudit: %v", err)
	}
	if store.called != 1 {
		t.Fatalf("expected 1 activity create, got %d", store.called)
	}
	if store.activity.CapturedBy != "human:audit-test" {
		t.Fatalf("expected captured_by to match caller-provided provenance, got %q", store.activity.CapturedBy)
	}
	if store.activity.Body == nil || *store.activity.Body != "Acme Corp" {
		t.Fatalf("expected source quote in activity body, got %v", store.activity.Body)
	}
	if store.activity.Subject == nil || !strings.Contains(*store.activity.Subject, "Extraction accepted: name") {
		t.Fatalf("expected extraction-accept subject, got %v", store.activity.Subject)
	}
	if len(store.activity.Links) != 1 || store.activity.Links[0].EntityType != domain.EntityTypeDeal || store.activity.Links[0].EntityID != "deal-1" {
		t.Fatalf("expected deal link, got %+v", store.activity.Links)
	}
}
