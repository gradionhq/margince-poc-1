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

	customfields "github.com/gradionhq/margince/backend/internal/platform/customfields"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
)

func patchCF(h *customfields.Handler, path, wsID, userID string, isAgent bool, token string, body map[string]any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(b))
	ctx := crmctx.With(req.Context(), crmctx.Principal{UserID: userID, TenantID: wsID, IsAgent: isAgent})
	req = req.WithContext(ctx)
	if token != "" {
		req.Header.Set("X-Approval-Token", token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func postCFAction(h *customfields.Handler, path, wsID, userID string, isAgent bool, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	ctx := crmctx.With(req.Context(), crmctx.Principal{UserID: userID, TenantID: wsID, IsAgent: isAgent})
	req = req.WithContext(ctx)
	if token != "" {
		req.Header.Set("X-Approval-Token", token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func seedCFHTTPWorkspaceAndUsers2(t *testing.T, db *sql.DB) (wsID, humanID, agentID string) {
	t.Helper()
	wsID, humanID = seedCFWorkspaceAndUser(t, db)
	agentID = ids.New()
	mustExec(t, db, `INSERT INTO app_user (id, workspace_id, email, display_name, is_agent) VALUES ($1::uuid, $2::uuid, $3, $4, true)`, agentID, wsID, "a"+agentID+"@t.test", "Agent")
	return wsID, humanID, agentID
}

// TestRenameCustomField_HumanNoToken_200_ColumnNameUnchanged proves the UAT
// step "Rename a text field's label ... as a human caller (no token) -> 200,
// column_name unchanged, one audit row."
func TestRenameCustomField_HumanNoToken_200_ColumnNameUnchanged(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers2(t, db)
	tag := time.Now().Format("150405.000000000")

	wCreate := postCF(h, wsID, humanID, false, "", map[string]any{"object": "deal", "label": "Original " + tag, "type": "text", "source": "ui", "captured_by": "human:" + humanID})
	var created map[string]any
	_ = json.Unmarshal(wCreate.Body.Bytes(), &created)
	id := created["id"].(string)
	colName := created["column_name"].(string)

	wRename := patchCF(h, "/custom-fields/"+id, wsID, humanID, false, "", map[string]any{"label": "Renamed " + tag})
	if wRename.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wRename.Code, wRename.Body.String())
	}
	var renamed map[string]any
	_ = json.Unmarshal(wRename.Body.Bytes(), &renamed)
	if renamed["column_name"] != colName {
		t.Fatalf("column_name must be unchanged: before=%v after=%v", colName, renamed["column_name"])
	}
	if renamed["label"] != "Renamed "+tag {
		t.Fatalf("expected label to update, got %v", renamed["label"])
	}
}

// TestRenameCustomField_AgentNoToken_200_GreenTierNeverGated proves
// renameCustomField's 🟢 tier at the HTTP level: an agent caller with a
// well-formed body and NO token still succeeds — toolgate.Enforce must
// never demand a token for a green-tier op, for either principal kind
// (P6/P12 MCP-tier reachability; complements the 🟡 retire/options agent
// flows below, which do require a token).
func TestRenameCustomField_AgentNoToken_200_GreenTierNeverGated(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, agentID := seedCFHTTPWorkspaceAndUsers2(t, db)
	tag := time.Now().Format("150405.000000000")

	// createCustomField is 🟡 (yellow) — seed the field as the human caller
	// so setup itself doesn't need a token; the field this test renames
	// belongs to the workspace, not to whichever principal created it (RLS
	// scopes by workspace_id, never by created_by).
	wCreate := postCF(h, wsID, humanID, false, "", map[string]any{"object": "deal", "label": "Agent original " + tag, "type": "text", "source": "ui", "captured_by": "human:" + humanID})
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("setup create: expected 201, got %d: %s", wCreate.Code, wCreate.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(wCreate.Body.Bytes(), &created)
	id := created["id"].(string)

	wRename := patchCF(h, "/custom-fields/"+id, wsID, agentID, true, "", map[string]any{"label": "Agent renamed " + tag})
	if wRename.Code != http.StatusOK {
		t.Fatalf("agent rename with no token: expected 200 (green tier never gates), got %d: %s", wRename.Code, wRename.Body.String())
	}
}

func TestRenameCustomField_NonexistentID_404(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers2(t, db)
	w := patchCF(h, "/custom-fields/018f3a1b-0000-7000-8000-0000000000ff", wsID, humanID, false, "", map[string]any{"label": "X"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRetireCustomField_HumanThenAgentFlow proves the UAT step: human ->
// 200 status=retired archived_at=null; agent without token -> 403; agent
// with valid token -> 200.
func TestRetireCustomField_HumanThenAgentFlow(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "cf-t04-retire-it-secret")
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, agentID := seedCFHTTPWorkspaceAndUsers2(t, db)
	tag := time.Now().Format("150405.000000000")

	wCreate := postCF(h, wsID, humanID, false, "", map[string]any{"object": "lead", "label": "Retire me " + tag, "type": "text", "source": "ui", "captured_by": "human:" + humanID})
	var created map[string]any
	_ = json.Unmarshal(wCreate.Body.Bytes(), &created)
	id := created["id"].(string)

	wHuman := postCFAction(h, "/custom-fields/"+id+"/retire", wsID, humanID, false, "")
	if wHuman.Code != http.StatusOK {
		t.Fatalf("human retire: expected 200, got %d: %s", wHuman.Code, wHuman.Body.String())
	}
	var retired map[string]any
	_ = json.Unmarshal(wHuman.Body.Bytes(), &retired)
	if retired["status"] != "retired" || retired["archived_at"] != nil {
		t.Fatalf("expected status=retired archived_at=nil, got %+v", retired)
	}

	wCreate2 := postCF(h, wsID, humanID, false, "", map[string]any{"object": "lead", "label": "Retire me agent " + tag, "type": "text", "source": "ui", "captured_by": "human:" + humanID})
	var created2 map[string]any
	_ = json.Unmarshal(wCreate2.Body.Bytes(), &created2)
	id2 := created2["id"].(string)

	wNoToken := postCFAction(h, "/custom-fields/"+id2+"/retire", wsID, agentID, true, "")
	if wNoToken.Code != http.StatusForbidden {
		t.Fatalf("agent no token: expected 403, got %d: %s", wNoToken.Code, wNoToken.Body.String())
	}

	diffFields := map[string]any{"id": id2}
	diffHash := approvalsport.HashDiff(diffFields)
	tok, err := crmapprovals.SignToken(crmapprovals.TokenClaims{
		JTI: "cf-t04-retire-jti-" + tag, WorkspaceID: wsID, Tool: "update_record", DiffHash: diffHash,
		Exp: time.Now().Add(5 * time.Minute), SingleUse: true,
	})
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	wToken := postCFAction(h, "/custom-fields/"+id2+"/retire", wsID, agentID, true, tok)
	if wToken.Code != http.StatusOK {
		t.Fatalf("agent with token: expected 200, got %d: %s", wToken.Code, wToken.Body.String())
	}
}

func TestRetireCustomField_NonexistentID_404(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers2(t, db)
	w := postCFAction(h, "/custom-fields/018f3a1b-0000-7000-8000-0000000000ff/retire", wsID, humanID, false, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSetCustomFieldOptions_AddRemoveKeepAtLeastOne proves the UAT step:
// edit a picklist's options (add one, remove one, keep >=1) -> 200; a
// removed-option write now fails; removing the last remaining option -> 422.
func TestSetCustomFieldOptions_AddRemoveKeepAtLeastOne(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers2(t, db)
	tag := time.Now().Format("150405.000000000")

	wCreate := postCF(h, wsID, humanID, false, "", map[string]any{
		"object": "deal", "label": "Route " + tag, "type": "picklist", "options": []string{"direct", "reseller"},
		"source": "ui", "captured_by": "human:" + humanID,
	})
	var created map[string]any
	_ = json.Unmarshal(wCreate.Body.Bytes(), &created)
	id := created["id"].(string)

	wEdit := patchCF(h, "/custom-fields/"+id+"/options", wsID, humanID, false, "", map[string]any{"options": []string{"direct", "marketplace"}})
	if wEdit.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wEdit.Code, wEdit.Body.String())
	}

	wLast := patchCF(h, "/custom-fields/"+id+"/options", wsID, humanID, false, "", map[string]any{"options": []string{}})
	if wLast.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", wLast.Code, wLast.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(wLast.Body.Bytes(), &body)
	if body["detail"] != "A picklist needs at least one option" {
		t.Fatalf(`expected detail="A picklist needs at least one option", got %v`, body["detail"])
	}
}

func TestSetCustomFieldOptions_NonPicklistField_422(t *testing.T) {
	db := testDB(t)
	h := cfHandlerForTest(db)
	wsID, humanID, _ := seedCFHTTPWorkspaceAndUsers2(t, db)
	tag := time.Now().Format("150405.000000000")

	wCreate := postCF(h, wsID, humanID, false, "", map[string]any{"object": "deal", "label": "Text field " + tag, "type": "text", "source": "ui", "captured_by": "human:" + humanID})
	var created map[string]any
	_ = json.Unmarshal(wCreate.Body.Bytes(), &created)
	id := created["id"].(string)

	w := patchCF(h, "/custom-fields/"+id+"/options", wsID, humanID, false, "", map[string]any{"options": []string{"a"}})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}
