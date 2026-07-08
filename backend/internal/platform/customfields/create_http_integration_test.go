//go:build integration

package customfields_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq" // registers the "postgres" database/sql driver

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	customfields "github.com/gradionhq/margince/backend/internal/platform/customfields"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
)

func seedCFHTTPWorkspaceAndUsers(t *testing.T, db *sql.DB) (wsID, humanID, agentID string) {
	t.Helper()
	wsID, humanID, agentID = ids.New(), ids.New(), ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "cfh-"+wsID, "cfh-"+wsID)
	mustExec(t, db, `INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`, humanID, wsID, "h"+humanID+"@t.test", "Human")
	mustExec(t, db, `INSERT INTO app_user (id,workspace_id,email,display_name,is_agent) VALUES ($1::uuid,$2::uuid,$3,$4,true)`, agentID, wsID, "a"+agentID+"@t.test", "Agent")
	return wsID, humanID, agentID
}

func cfHandlerForTest(db *sql.DB) *customfields.Handler {
	return customfields.NewHandler(db, &crmapprovals.DBVerifier{DB: db})
}

func postCF(h *customfields.Handler, wsID, userID string, isAgent bool, token string, body map[string]any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/custom-fields", bytes.NewReader(b))
	ctx := crmctx.With(req.Context(), crmctx.Principal{UserID: userID, TenantID: wsID, IsAgent: isAgent})
	req = req.WithContext(ctx)
	if token != "" {
		req.Header.Set("X-Approval-Token", token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// cfTypeCase is one row of TestCreateCustomField_AllSixTypes_EndToEnd's
// type-to-column table.
type cfTypeCase struct {
	name    string
	fType   string
	object  string
	extra   map[string]any
	wantSQL string
}

// assertCreateCustomFieldCase runs a single cfTypeCase end to end: POST the
// field, then assert the resulting pg column type, the ISO-4217 currency
// catalog row (currency type only), and the catalog+audit row counts.
// Extracted out of TestCreateCustomField_AllSixTypes_EndToEnd's t.Run
// closure to keep the parent test's cognitive complexity under the lint
// threshold — every assertion below is unchanged from the inline version.
func assertCreateCustomFieldCase(t *testing.T, h *customfields.Handler, db *sql.DB, wsID, humanID string, c cfTypeCase) {
	t.Helper()
	body := map[string]any{"object": c.object, "label": c.name, "type": c.fType, "source": "ui", "captured_by": "human:" + humanID}
	for k, v := range c.extra {
		body[k] = v
	}
	w := postCF(h, wsID, humanID, false, "", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	colName := created["column_name"].(string)

	var pgType string
	mustQueryScalar(t, db, &pgType, `SELECT data_type FROM information_schema.columns WHERE table_name=$1 AND column_name=$2`, c.object, colName)
	if pgType != c.wantSQL {
		t.Errorf("type=%s: pg column type = %q, want %q", c.fType, pgType, c.wantSQL)
	}

	// CUSTOM-FIELDS-AC-11 explicitly requires the ISO-4217 code land
	// in the catalog row (the column itself is bare bigint minor-units).
	if c.fType == "currency" {
		var storedCurrency string
		mustQueryScalar(t, db, &storedCurrency, `SELECT currency FROM custom_field WHERE id=$1::uuid`, created["id"])
		if storedCurrency != "USD" {
			t.Errorf("currency: catalog row currency = %q, want USD", storedCurrency)
		}
	}

	var catalogCount, auditCount int
	mustQueryScalar(t, db, &catalogCount, `SELECT count(*) FROM custom_field WHERE id=$1::uuid`, created["id"])
	mustQueryScalar(t, db, &auditCount, `SELECT count(*) FROM audit_log WHERE entity_id=$1::uuid AND action='create' AND entity_type='custom_field'`, created["id"])
	if catalogCount != 1 || auditCount != 1 {
		t.Errorf("type=%s: catalog=%d audit=%d, want 1/1", c.fType, catalogCount, auditCount)
	}
}

// TestCreateCustomField_AllSixTypes_EndToEnd proves CUSTOM-FIELDS-AC-11: the
// six types map exactly per PARAM-4, and each creates a real, queryable
// column plus matching catalog + audit rows.
func TestCreateCustomField_AllSixTypes_EndToEnd(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers(t, db)
	tag := time.Now().Format("150405.000000000")

	cases := []cfTypeCase{
		{"Text field " + tag, "text", "person", nil, "text"},
		{"Number field " + tag, "number", "person", nil, "numeric"},
		{"Date field " + tag, "date", "deal", nil, "date"},
		{"Currency field " + tag, "currency", "deal", map[string]any{"currency": "USD"}, "bigint"},
		{"Picklist field " + tag, "picklist", "deal", map[string]any{"options": []string{"direct", "reseller"}}, "text"},
		{"Boolean field " + tag, "boolean", "activity", nil, "boolean"},
	}
	for _, c := range cases {
		t.Run(c.fType, func(t *testing.T) {
			assertCreateCustomFieldCase(t, h, db, wsID, humanID, c)
		})
	}
}

func TestCreateCustomField_UnsupportedTypeOrObject_422(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers(t, db)

	w1 := postCF(h, wsID, humanID, false, "", map[string]any{"object": "deal", "label": "X", "type": "money", "source": "ui", "captured_by": "human:" + humanID})
	if w1.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unsupported type: expected 422, got %d: %s", w1.Code, w1.Body.String())
	}
	w2 := postCF(h, wsID, humanID, false, "", map[string]any{"object": "widget", "label": "X", "type": "text", "source": "ui", "captured_by": "human:" + humanID})
	if w2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unsupported object: expected 422, got %d: %s", w2.Code, w2.Body.String())
	}
}

// TestCreateCustomField_ValidationErrorBody_MatchesContractShape proves the
// wire body's details.errors[] items are {field, code} (lowercase), not the
// Go struct's capitalised field names — the FieldError json tags are
// load-bearing (plan-review B1).
func TestCreateCustomField_ValidationErrorBody_MatchesContractShape(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers(t, db)

	w := postCF(h, wsID, humanID, false, "", map[string]any{"object": "deal", "label": "Budget ceiling", "type": "currency", "source": "ui", "captured_by": "human:" + humanID})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "validation_error" {
		t.Fatalf("expected code=validation_error, got %v", body["code"])
	}
	details, _ := body["details"].(map[string]any)
	errs, _ := details["errors"].([]any)
	if len(errs) == 0 {
		t.Fatalf("expected at least one validation error, got body=%v", body)
	}
	first, _ := errs[0].(map[string]any)
	if first["field"] != "currency" || first["code"] != "required_for_type_currency" {
		t.Fatalf("expected the first error to be {field:currency, code:required_for_type_currency} (lowercase keys), got %v", first)
	}
}

// TestCreateCustomField_StructuralLabel_422 proves CUSTOM-FIELDS-AC-4/AC-8
// and the WIRE-5 exact problem shape.
func TestCreateCustomField_StructuralLabel_422(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers(t, db)

	w := postCF(h, wsID, humanID, false, "", map[string]any{"object": "deal", "label": "Link to another object", "type": "text", "source": "ui", "captured_by": "human:" + humanID})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "structural_change_refused" {
		t.Fatalf("expected code=structural_change_refused, got %v", body["code"])
	}
	details, _ := body["details"].(map[string]any)
	if details["route"] != "source_development_path" {
		t.Fatalf("expected details.route=source_development_path, got %v", body["details"])
	}
}

// TestCreateCustomField_InjectionAttemptLabel_NeverReachesRawSQL is the
// HTTP-level round trip for CUSTOM-FIELDS-AC-12: the malicious label is
// stored verbatim as the display label, but the physical column and
// generated DDL only ever used the derived-safe identifier, and the
// target table survives intact.
func TestCreateCustomField_InjectionAttemptLabel_NeverReachesRawSQL(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers(t, db)

	// Timestamp-tagged so the derived slug/column name is unique per run —
	// otherwise a fixed malicious label derives the same cf_ column every
	// time and a second run collides on the already-added column.
	tag := time.Now().Format("150405.000000000")
	label := `evil field ` + tag + `'); DROP TABLE person;--`
	w := postCF(h, wsID, humanID, false, "", map[string]any{"object": "person", "label": label, "type": "text", "source": "ui", "captured_by": "human:" + humanID})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 (sanitized, not rejected outright), got %d: %s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	colName := created["column_name"].(string)
	for _, bad := range []string{"DROP", "--", "'", ";"} {
		if bytesContains(colName, bad) {
			t.Fatalf("column_name leaked raw label text: %s", colName)
		}
	}
	var personExists bool
	mustQueryScalar(t, db, &personExists, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='person')`)
	if !personExists {
		t.Fatal("person table must still exist — injection must never reach raw SQL")
	}
	var storedLabel string
	mustQueryScalar(t, db, &storedLabel, `SELECT label FROM custom_field WHERE id=$1::uuid`, created["id"])
	if storedLabel != label {
		t.Fatalf("catalog label must preserve the original text for display: got %q want %q", storedLabel, label)
	}
}

func bytesContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCreateCustomField_HumanBypassesGate_AgentRequiresToken proves RC-12 +
// the toolgate contract end to end: human never needs a token; agent
// without one gets 403; agent with a valid single-use token succeeds and a
// replay is rejected.
func TestCreateCustomField_HumanBypassesGate_AgentRequiresToken(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "cf-handler-it-secret")
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, agentID := seedCFHTTPWorkspaceAndUsers(t, db)
	tag := time.Now().Format("150405.000000000")

	// Human: no token, succeeds.
	wHuman := postCF(h, wsID, humanID, false, "", map[string]any{"object": "lead", "label": "Human field " + tag, "type": "text", "source": "ui", "captured_by": "human:" + humanID})
	if wHuman.Code != http.StatusCreated {
		t.Fatalf("human bypass: expected 201, got %d: %s", wHuman.Code, wHuman.Body.String())
	}

	// Agent, no token: 403 approval_required, nothing written.
	agentBody := map[string]any{"object": "lead", "label": "Agent field " + tag, "type": "text", "source": "agent", "captured_by": "agent:" + agentID}
	wNoToken := postCF(h, wsID, agentID, true, "", agentBody)
	if wNoToken.Code != http.StatusForbidden {
		t.Fatalf("agent no token: expected 403, got %d: %s", wNoToken.Code, wNoToken.Body.String())
	}
	var n int
	mustQueryScalar(t, db, &n, `SELECT count(*) FROM custom_field WHERE workspace_id=$1::uuid AND label=$2`, wsID, "Agent field "+tag)
	if n != 0 {
		t.Fatalf("agent without token must write nothing, found %d rows", n)
	}

	// Agent, valid single-use token: succeeds; replay rejected.
	diffFields := map[string]any{"object": "lead", "label": "Agent field " + tag, "type": "text"}
	diffHash := approvalsport.HashDiff(diffFields)
	tok, err := crmapprovals.SignToken(crmapprovals.TokenClaims{
		JTI: "cf-t03-jti-" + tag, WorkspaceID: wsID, Tool: "create_record", DiffHash: diffHash,
		Exp: time.Now().Add(5 * time.Minute), SingleUse: true,
	})
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	wToken := postCF(h, wsID, agentID, true, tok, agentBody)
	if wToken.Code != http.StatusCreated {
		t.Fatalf("agent with valid token: expected 201, got %d: %s", wToken.Code, wToken.Body.String())
	}
	wReplay := postCF(h, wsID, agentID, true, tok, agentBody)
	if wReplay.Code != http.StatusForbidden {
		t.Fatalf("token replay: expected 403, got %d: %s", wReplay.Code, wReplay.Body.String())
	}
}

// TestNoFieldMetadataOrEAVTableBacksCustomFields is the NEVER-1/DM-CONV-16
// schema test: custom fields are real columns; no sidecar metadata/EAV/JSON
// table exists.
func TestNoFieldMetadataOrEAVTableBacksCustomFields(t *testing.T) {
	db := testDB(t)
	for _, forbidden := range []string{"field_metadata", "custom_field_value", "organization_custom", "person_custom", "deal_custom", "lead_custom", "activity_custom"} {
		var exists bool
		mustQueryScalar(t, db, &exists, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name=$1)`, forbidden)
		if exists {
			t.Fatalf("forbidden sidecar table %q exists — custom fields must be real columns, not EAV/metadata rows", forbidden)
		}
	}
}
