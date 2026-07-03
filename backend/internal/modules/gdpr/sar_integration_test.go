//go:build integration

package crmgdpr_test

import (
	"context"
	"testing"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// TestAssemble_CompleteSARPackage verifies that Assemble returns data from all linked objects
// and writes exactly one audit_log row with action=export.
func TestAssemble_CompleteSARPackage(t *testing.T) {
	db := testDB(t)
	wsID, personID := seedWorkspaceAndPerson(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "admin-user", TenantID: wsID})

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// Seed a pipeline + stage for deal creation.
	pipelineID := ids.New()
	stageID := ids.New()
	mustExec(t, db,
		`INSERT INTO pipeline (id,workspace_id,name,is_default,position) VALUES ($1::uuid,$2::uuid,'SAR Pipeline',true,1)`,
		pipelineID, wsID)
	mustExec(t, db,
		`INSERT INTO stage (id,workspace_id,pipeline_id,name,position,semantic,win_probability) VALUES ($1::uuid,$2::uuid,$3::uuid,'Open',1,'open',0)`,
		stageID, wsID, pipelineID)

	// Seed an email.
	mustExec(t, db, `INSERT INTO person_email (id,workspace_id,person_id,email,source,captured_by)
		VALUES ($1::uuid,$2::uuid,$3::uuid,'sar-test@example.com','test','test')`,
		ids.New(), wsID, personID)

	// Seed an activity linked to the person.
	activityID := ids.New()
	mustExec(t, db, `INSERT INTO activity (id,workspace_id,kind,subject,body,raw,source,captured_by)
		VALUES ($1::uuid,$2::uuid,'note','SAR meeting','Agenda notes','{"key":"val"}'::jsonb,'test','test')`,
		activityID, wsID)
	mustExec(t, db, `INSERT INTO activity_link (id,workspace_id,activity_id,entity_type,person_id)
		VALUES ($1::uuid,$2::uuid,$3::uuid,'person',$4::uuid)`,
		ids.New(), wsID, activityID, personID)

	// Seed an organization and a deal linked via activity_link.
	orgID := ids.New()
	mustExec(t, db, `INSERT INTO organization (id,workspace_id,name,source,captured_by)
		VALUES ($1::uuid,$2::uuid,'SAR Org','test','test')`,
		orgID, wsID)

	dealID := ids.New()
	mustExec(t, db,
		`INSERT INTO deal (id,workspace_id,name,pipeline_id,stage_id,organization_id,source,captured_by)
		 VALUES ($1::uuid,$2::uuid,'SAR Deal',$3::uuid,$4::uuid,$5::uuid,'test','test')`,
		dealID, wsID, pipelineID, stageID, orgID)

	// Link activity → deal via activity_link.
	mustExec(t, db, `INSERT INTO activity_link (id,workspace_id,activity_id,entity_type,deal_id)
		VALUES ($1::uuid,$2::uuid,$3::uuid,'deal',$4::uuid)`,
		ids.New(), wsID, activityID, dealID)

	// Seed raw capture in person.raw.
	mustExec(t, db, `UPDATE person SET raw='{"src":"hubspot","contact_id":"123"}'::jsonb WHERE id=$1::uuid`, personID)

	// Call the actual exported symbol under test.
	pkg, err := crmgdpr.Assemble(ctx, db, personID)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	if len(pkg.Person) == 0 {
		t.Error("SARPackage.Person: must not be empty")
	}
	if len(pkg.Emails) == 0 {
		t.Error("SARPackage.Emails: want ≥1, got 0")
	}
	if len(pkg.Activities) == 0 {
		t.Error("SARPackage.Activities: want ≥1, got 0")
	}
	if len(pkg.Deals) == 0 {
		t.Error("SARPackage.Deals: want ≥1, got 0")
	}
	if len(pkg.Organizations) == 0 {
		t.Error("SARPackage.Organizations: want ≥1, got 0")
	}
	if len(pkg.RawCapture) == 0 {
		t.Error("SARPackage.RawCapture: want ≥1 (person raw), got 0")
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// Exactly one audit_log row with action=export for this person.
	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log
		 WHERE action='export' AND entity_type='person'
		   AND entity_id=$1::uuid AND workspace_id=$2::uuid`,
		personID, wsID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("audit_log export rows: want 1, got %d", auditCount)
	}

	// Audit After must be non-null — manifest with counts, not a PII dump.
	var afterText *string
	if err := db.QueryRow(
		`SELECT after::text FROM audit_log
		 WHERE action='export' AND entity_type='person' AND entity_id=$1::uuid AND workspace_id=$2::uuid`,
		personID, wsID,
	).Scan(&afterText); err != nil {
		t.Fatalf("query audit_log after: %v", err)
	}
	if afterText == nil {
		t.Error("audit_log.after: must be non-null (manifest)")
	}
}
