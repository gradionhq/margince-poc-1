//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const dealPartnerTestWorkspaceID = "00000000-0000-0000-0000-000000000006"

// seedDealPartnerFixtures seeds a pipeline/stage and two partner orgs (both with a
// live partner row, so DealStore.Update's partner_org_id FK target is valid).
func seedDealPartnerFixtures(t *testing.T, db *sql.DB, tag string) (pipelineID, stageID, orgAID, orgBID string) {
	t.Helper()
	tag = fmt.Sprintf("%s-%d", tag, time.Now().UnixNano())
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t15-deal-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, dealPartnerTestWorkspaceID, "t15-deal-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, dealPartnerTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	var pipelineID_, stageID_ string
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		dealPartnerTestWorkspaceID, "P-"+tag).Scan(&pipelineID_); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position) VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`,
		dealPartnerTestWorkspaceID, pipelineID_, "S-"+tag).Scan(&stageID_); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	var orgA, orgB string
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, classification, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'partner', 'test', 'human:test') RETURNING id`, dealPartnerTestWorkspaceID, "OrgA-"+tag).Scan(&orgA); err != nil {
		t.Fatalf("seed org A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, classification, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'partner', 'test', 'human:test') RETURNING id`, dealPartnerTestWorkspaceID, "OrgB-"+tag).Scan(&orgB); err != nil {
		t.Fatalf("seed org B: %v", err)
	}
	for _, orgID := range []string{orgA, orgB} {
		if _, err := db.Exec(`INSERT INTO partner (id, workspace_id, organization_id, cert_status, source, captured_by)
			VALUES (uuidv7(), $1, $2, 'applied', 'test', 'human:test')`, dealPartnerTestWorkspaceID, orgID); err != nil {
			t.Fatalf("seed partner row for org %s: %v", orgID, err)
		}
	}
	return pipelineID_, stageID_, orgA, orgB
}

func TestDealStore_Update_PersistsPartnerOrgIDChangeWithAudit(t *testing.T) {
	db := openTestDB(t)
	pipelineID, stageID, orgA, orgB := seedDealPartnerFixtures(t, db, "update")
	store := adapters.NewDealStore(db)

	d := domain.NewDeal("Deal with partner", pipelineID, stageID, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = dealPartnerTestWorkspaceID
	d.PartnerOrgID = &orgA
	created, err := store.Create(context.Background(), d, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.PartnerOrgID == nil || *created.PartnerOrgID != orgA {
		t.Fatalf("created.PartnerOrgID = %v, want %s", created.PartnerOrgID, orgA)
	}

	updated, err := store.Update(context.Background(), created.ID, dealPartnerTestWorkspaceID,
		map[string]any{"partner_org_id": orgB}, 0)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.PartnerOrgID == nil || *updated.PartnerOrgID != orgB {
		t.Fatalf("updated.PartnerOrgID = %v, want %s", updated.PartnerOrgID, orgB)
	}

	var auditCount int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity_type='deal' AND entity_id=$1::uuid AND action='update'`,
		created.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit_log 'update' rows for deal %s = %d, want 1", created.ID, auditCount)
	}

	var beforeRaw []byte
	if err := db.QueryRow(`SELECT before FROM audit_log WHERE entity_type='deal' AND entity_id=$1::uuid AND action='update'`,
		created.ID).Scan(&beforeRaw); err != nil {
		t.Fatalf("read audit before: %v", err)
	}
	var before map[string]any
	if err := json.Unmarshal(beforeRaw, &before); err != nil {
		t.Fatalf("unmarshal audit before: %v", err)
	}
	if before["partner_org_id"] != orgA {
		t.Fatalf("audit before.partner_org_id = %v, want %s", before["partner_org_id"], orgA)
	}

	var eventCount int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='deal.partner_assigned' AND entity_id=$1::uuid`,
		created.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("event_outbox 'deal.partner_assigned' rows = %d, want 1", eventCount)
	}
}
