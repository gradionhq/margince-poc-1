//go:build integration

package transport

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
	reltransport "github.com/gradionhq/margince/backend/internal/modules/relationships/transport"
)

func seedStakeholderDealFixtures(t *testing.T, db *sql.DB, tag string) (dealID, personA, personB string) {
	t.Helper()
	pipelineID, stageID, _ := seedDealFixtures(t, db, "stakeholders-"+tag)
	if err := db.QueryRow(`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, status, source, captured_by, version)
		VALUES (uuidv7(), $1, $2, $3, $4, 'open', 'test', 'human:test', 1) RETURNING id`,
		dealTestWorkspaceID, "Stakeholder Deal "+tag, pipelineID, stageID).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO person (id, workspace_id, full_name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`, dealTestWorkspaceID, "Champion "+tag).Scan(&personA); err != nil {
		t.Fatalf("seed person A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO person (id, workspace_id, full_name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`, dealTestWorkspaceID, "Blocker "+tag).Scan(&personB); err != nil {
		t.Fatalf("seed person B: %v", err)
	}
	return dealID, personA, personB
}

func TestDealHandler_ListStakeholders_ReturnsBothRoles_DuplicateRoleConflicts(t *testing.T) {
	db := openDealTestDB(t)
	dealID, personA, personB := seedStakeholderDealFixtures(t, db, time.Now().Format("150405.000000000"))
	relStore := relationships.NewRelationshipStore(db)
	dealHandler := dealHandlerForTest(db, adapters.NewDealStore(db))

	create := func(personID, role string) *httptest.ResponseRecorder {
		relHandler := reltransport.NewRelationshipHandler(relStore)
		body, _ := json.Marshal(map[string]any{
			"kind": "deal_stakeholder", "deal_id": dealID, "person_id": personID, "role": role,
			"source": "test", "captured_by": "human:test",
		})
		req := withDealWorkspace(httptest.NewRequest(http.MethodPost, "/relationships", bytes.NewReader(body)))
		w := httptest.NewRecorder()
		relHandler.ServeHTTP(w, req)
		return w
	}

	if w := create(personA, "champion"); w.Code != http.StatusCreated {
		t.Fatalf("create champion status = %d, body=%s", w.Code, w.Body.String())
	}
	if w := create(personB, "blocker"); w.Code != http.StatusCreated {
		t.Fatalf("create blocker status = %d, body=%s", w.Code, w.Body.String())
	}
	if w := create(personA, "champion"); w.Code != http.StatusConflict {
		t.Fatalf("duplicate (deal,person,role) status = %d, want 409, body=%s", w.Code, w.Body.String())
	}

	listReq := withDealWorkspace(httptest.NewRequest(http.MethodGet, "/deals/"+dealID+"/stakeholders", nil))
	listW := httptest.NewRecorder()
	dealHandler.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list stakeholders status = %d, body=%s", listW.Code, listW.Body.String())
	}
	var page struct {
		Data []relationships.Relationship `json:"data"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Data) != 2 {
		t.Fatalf("expected 2 stakeholders, got %d: %+v", len(page.Data), page.Data)
	}
}

func TestDealHandler_ListStakeholders_UnknownDeal_Returns404(t *testing.T) {
	db := openDealTestDB(t)
	seedDealFixtures(t, db, "unknown-deal")
	dealHandler := dealHandlerForTest(db, adapters.NewDealStore(db))

	req := withDealWorkspace(httptest.NewRequest(http.MethodGet, "/deals/00000000-0000-0000-0000-0000000000ff/stakeholders", nil))
	w := httptest.NewRecorder()
	dealHandler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", w.Code, w.Body.String())
	}
}
