//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	adapters "github.com/gradionhq/margince/backend/internal/modules/partners/adapters"
	domain "github.com/gradionhq/margince/backend/internal/modules/partners/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func openPartnerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

const partnerTestWorkspaceID = "00000000-0000-0000-0000-000000000004"

func seedPartnerOrgFixture(t *testing.T, db *sql.DB, tag string) string {
	t.Helper()
	tag = fmt.Sprintf("%s-%d", tag, time.Now().UnixNano())
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t15-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, partnerTestWorkspaceID, "t15-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, partnerTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	var orgID string
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`,
		partnerTestWorkspaceID, "Org "+tag).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	return orgID
}

func TestPartnerStore_Upsert_CreatesThenUpdatesSameRowAndSetsOrgClassification(t *testing.T) {
	db := openPartnerTestDB(t)
	orgID := seedPartnerOrgFixture(t, db, "upsert")
	store := adapters.NewPartnerStore(db)

	role := "hosting"
	p := domain.NewPartner(orgID, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = partnerTestWorkspaceID
	p.PartnerRole = &role
	p.CertStatus = "applied"

	created, err := store.Upsert(context.Background(), p)
	if err != nil {
		t.Fatalf("Upsert create: %v", err)
	}
	if created.CertStatus != "applied" {
		t.Fatalf("cert_status = %q, want applied", created.CertStatus)
	}

	var classification string
	if err := db.QueryRow(`SELECT classification FROM organization WHERE id=$1::uuid`, orgID).Scan(&classification); err != nil {
		t.Fatalf("read org classification: %v", err)
	}
	if classification != "partner" {
		t.Fatalf("org.classification = %q, want partner", classification)
	}

	p2 := p
	p2.ID = created.ID
	p2.CertStatus = "certified"
	updated, err := store.Upsert(context.Background(), p2)
	if err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("upsert duplicated the row: got id %s, want %s", updated.ID, created.ID)
	}
	if updated.CertStatus != "certified" {
		t.Fatalf("cert_status after update = %q, want certified", updated.CertStatus)
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM partner WHERE organization_id=$1::uuid`, orgID).Scan(&count); err != nil {
		t.Fatalf("count partner rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("partner row count = %d, want 1 (1:1 org extension)", count)
	}

	var auditCount int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity_type='partner' AND entity_id=$1::uuid`, created.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 2 {
		t.Fatalf("audit_log rows for partner %s = %d, want 2 (create + update)", created.ID, auditCount)
	}

	var eventCount int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic IN ('partner.created','partner.updated') AND entity_id=$1::uuid`, created.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if eventCount != 2 {
		t.Fatalf("event_outbox rows for partner %s = %d, want 2", created.ID, eventCount)
	}
}

func TestPartnerStore_Get_NotFoundWhenNoPartnerRow(t *testing.T) {
	db := openPartnerTestDB(t)
	orgID := seedPartnerOrgFixture(t, db, "get-404")
	store := adapters.NewPartnerStore(db)

	_, err := store.Get(context.Background(), orgID, partnerTestWorkspaceID)
	if err == nil {
		t.Fatal("expected ErrNotFound for an org with no partner row")
	}
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestPartnerStore_List_FiltersByRoleAndCertStatus(t *testing.T) {
	db := openPartnerTestDB(t)
	store := adapters.NewPartnerStore(db)

	hosting := "hosting"
	consulting := "consulting"
	orgA := seedPartnerOrgFixture(t, db, "list-a")
	orgB := seedPartnerOrgFixture(t, db, "list-b")

	pa := domain.NewPartner(orgA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	pa.WorkspaceID = partnerTestWorkspaceID
	pa.PartnerRole = &hosting
	pa.CertStatus = "certified"
	if _, err := store.Upsert(context.Background(), pa); err != nil {
		t.Fatalf("seed partner A: %v", err)
	}

	pb := domain.NewPartner(orgB, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	pb.WorkspaceID = partnerTestWorkspaceID
	pb.PartnerRole = &consulting
	pb.CertStatus = "applied"
	if _, err := store.Upsert(context.Background(), pb); err != nil {
		t.Fatalf("seed partner B: %v", err)
	}

	items, _, err := store.List(context.Background(), partnerTestWorkspaceID, "", 20,
		domain.PartnerListFilter{PartnerRole: "hosting", CertStatus: "certified"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, it := range items {
		if it.OrganizationID == orgB {
			t.Fatalf("List returned org B, which doesn't match the filter")
		}
	}
	found := false
	for _, it := range items {
		if it.OrganizationID == orgA {
			found = true
		}
	}
	if !found {
		t.Fatal("List did not return org A, which matches the filter")
	}
}
