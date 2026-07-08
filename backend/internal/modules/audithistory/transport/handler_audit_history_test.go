//go:build integration

package transport_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/audithistory/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
	audithistorytransport "github.com/gradionhq/margince/backend/internal/modules/audithistory/transport"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/authz"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// seedAuditRow writes a raw audit_log row for a given entity, bypassing the RLS GUC
// by using the existing test-level SET session variable that setRLS already applies.
func seedAuditRow(t *testing.T, db *sql.DB, wsID, entityType, entityID, actorType, actorID string, onBehalfOf *string, action string) {
	t.Helper()
	e := crmaudit.Entry{
		WorkspaceID: wsID,
		ActorType:   actorType,
		ActorID:     actorID,
		OnBehalfOf:  onBehalfOf,
		Action:      action,
		EntityType:  entityType,
		EntityID:    &entityID,
		Before:      map[string]any{"stage": "Discovery", "salary": 120000},
		After:       map[string]any{"stage": "Proposal", "salary": 130000},
	}
	_, err := crmaudit.Write(context.Background(), db, e)
	if err != nil {
		t.Fatalf("seedAuditRow: %v", err)
	}
}

// seedAppUserForHistory seeds an app_user for on_behalf_of FK resolution.
func seedAppUserForHistory(t *testing.T, db *sql.DB, id, wsID, displayName string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO app_user(id, workspace_id, email, display_name)
		 VALUES($1::uuid, $2::uuid, $3, $4)
		 ON CONFLICT DO NOTHING`,
		id, wsID, "user-"+id+"@example.com", displayName)
	if err != nil {
		t.Fatalf("seedAppUserForHistory: %v", err)
	}
}

// noopAuthz is an Authorizer that always allows.
func noopAuthz(_ context.Context, _, _ string) error { return nil }

// denyAuthz is an Authorizer that always denies.
func denyAuthz(_ context.Context, _, _ string) error { return errors.New("forbidden") }

// TestHistoryHandler_AC1_ActorAndAuthorityRender covers AC1: every returned row has a
// non-empty actor; agent rows carry their on_behalf_of authority.
func TestHistoryHandler_AC1_ActorAndAuthorityRender(t *testing.T) {
	db := pgtest.OpenTestDB(t)

	const wsID = "00000000-0000-0000-0000-000000000101"
	const humanUserID = "00000000-0000-0000-0000-000000000201"
	const agentUserID = "00000000-0000-0000-0000-000000000202"
	const oboUserID = "00000000-0000-0000-0000-000000000203"
	const entityType = "deal"
	const entityID = "00000000-0000-0000-0000-000000000301"

	// Seed workspace
	db.ExecContext(context.Background(),
		`INSERT INTO workspace(id, name, slug, base_currency) VALUES($1,'histtest','hist-101','EUR') ON CONFLICT DO NOTHING`,
		wsID)

	// Seed app_users
	seedAppUserForHistory(t, db, humanUserID, wsID, "Alice Human")
	seedAppUserForHistory(t, db, agentUserID, wsID, "BotAgent")
	seedAppUserForHistory(t, db, oboUserID, wsID, "Devin Authority")

	// Set RLS for audit writes
	pgtest.SetRLS(t, db, wsID)

	// Seed a human mutation
	seedAuditRow(t, db, wsID, entityType, entityID, "human", humanUserID, nil, "update")

	// Seed an agent mutation on behalf of oboUserID
	oboStr := oboUserID
	seedAuditRow(t, db, wsID, entityType, entityID, "agent", agentUserID, &oboStr, "advance_stage")

	// Wait a moment to ensure ordering (uuidv7 monotonic + occurred_at)
	time.Sleep(2 * time.Millisecond)

	reader := adapters.NewAuditHistoryReader(db)
	h := audithistorytransport.NewHistoryHandler(reader, authz.Authorizer(noopAuthz))

	req := httptest.NewRequest(http.MethodGet,
		"/records/"+entityType+"/"+entityID+"/history", nil)
	ctx := crmctx.With(req.Context(), crmctx.Principal{TenantID: wsID, UserID: humanUserID})
	req = req.WithContext(ctx)
	// Inject path values the handler reads from the URL path:
	req.SetPathValue("entity_type", entityType)
	req.SetPathValue("id", entityID)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []domain.AuditHistoryEntry `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) < 2 {
		t.Fatalf("want at least 2 entries, got %d", len(resp.Data))
	}

	for _, entry := range resp.Data {
		if entry.ActorID == "" {
			t.Errorf("entry id=%s has empty actor_id", entry.ID)
		}
		if entry.Summary == "" {
			t.Errorf("entry id=%s has empty summary", entry.ID)
		}
	}

	// Find the agent row and assert on_behalf_of_name is set
	var agentEntry *domain.AuditHistoryEntry
	for i := range resp.Data {
		if resp.Data[i].ActorType == "agent" {
			agentEntry = &resp.Data[i]
			break
		}
	}
	if agentEntry == nil {
		t.Fatal("expected an agent entry in the history")
	}
	if agentEntry.OnBehalfOf == nil || *agentEntry.OnBehalfOf != oboUserID {
		t.Errorf("agent entry on_behalf_of want %q, got %v", oboUserID, agentEntry.OnBehalfOf)
	}
	if agentEntry.OnBehalfOfName == nil || *agentEntry.OnBehalfOfName != "Devin Authority" {
		t.Errorf("agent entry on_behalf_of_name want 'Devin Authority', got %v", agentEntry.OnBehalfOfName)
	}
	if !strings.Contains(agentEntry.Summary, "Devin Authority") {
		t.Errorf("agent summary must mention authority name, got %q", agentEntry.Summary)
	}
}

// TestHistoryHandler_AC2_FieldMaskingAndObjectLevelDeny covers AC2:
//   - masked field is absent from before/after
//   - viewer lacking read on the entity type gets 403
func TestHistoryHandler_AC2_FieldMaskingAndObjectLevelDeny(t *testing.T) {
	db := pgtest.OpenTestDB(t)

	const wsID = "00000000-0000-0000-0000-000000000102"
	const humanUserID = "00000000-0000-0000-0000-000000000204"
	const entityType = "deal"
	const entityID = "00000000-0000-0000-0000-000000000302"

	db.ExecContext(context.Background(),
		`INSERT INTO workspace(id, name, slug, base_currency) VALUES($1,'hist102','hist-102','EUR') ON CONFLICT DO NOTHING`,
		wsID)
	seedAppUserForHistory(t, db, humanUserID, wsID, "Bob Viewer")
	pgtest.SetRLS(t, db, wsID)

	seedAuditRow(t, db, wsID, entityType, entityID, "human", humanUserID, nil, "update")

	// AC2a: field masking — inject a mask that hides "salary" for "deal"
	masks := map[string]domain.EntityFieldMask{
		entityType: {"salary": struct{}{}},
	}
	reader := adapters.NewAuditHistoryReader(db).WithFieldMasks(masks)
	h := audithistorytransport.NewHistoryHandler(reader, authz.Authorizer(noopAuthz))

	req := httptest.NewRequest(http.MethodGet,
		"/records/"+entityType+"/"+entityID+"/history", nil)
	ctx := crmctx.With(req.Context(), crmctx.Principal{TenantID: wsID, UserID: humanUserID})
	req = req.WithContext(ctx)
	req.SetPathValue("entity_type", entityType)
	req.SetPathValue("id", entityID)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []domain.AuditHistoryEntry `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("want at least one entry")
	}
	entry := resp.Data[0]
	if entry.Before != nil {
		if _, hasSalary := entry.Before["salary"]; hasSalary {
			t.Error("masked field 'salary' must be absent from before")
		}
	}
	if entry.After != nil {
		if _, hasSalary := entry.After["salary"]; hasSalary {
			t.Error("masked field 'salary' must be absent from after")
		}
	}

	// AC2b: object-level deny — denyAuthz must return 403
	hDeny := audithistorytransport.NewHistoryHandler(reader, authz.Authorizer(denyAuthz))
	req2 := httptest.NewRequest(http.MethodGet,
		"/records/"+entityType+"/"+entityID+"/history", nil)
	ctx2 := crmctx.With(req2.Context(), crmctx.Principal{TenantID: wsID, UserID: humanUserID})
	req2 = req2.WithContext(ctx2)
	req2.SetPathValue("entity_type", entityType)
	req2.SetPathValue("id", entityID)

	w2 := httptest.NewRecorder()
	hDeny.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Fatalf("want 403 from denyAuthz, got %d", w2.Code)
	}
}
