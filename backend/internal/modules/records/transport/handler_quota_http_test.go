//go:build integration

package transport

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	identitytransport "github.com/gradionhq/margince/backend/internal/modules/identity/transport"
	"github.com/gradionhq/margince/backend/internal/modules/records"
	platformauth "github.com/gradionhq/margince/backend/internal/platform/auth"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// ---------- wire ----------

// qhHandler returns the production handler backed by a real QuotaStore.
func qhHandler(db *sql.DB) *QuotaHandler {
	return NewQuotaHandler(records.NewQuotaStore(db))
}

// qhDo sends an HTTP request to h with the given workspace+user principal injected.
func qhDo(t *testing.T, h http.Handler, method, path, body, wsID, userID string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{TenantID: wsID, UserID: userID}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// qhDoIfMatch is qhDo with an additional If-Match header.
func qhDoIfMatch(t *testing.T, h http.Handler, method, path, body, wsID, userID string, ifMatch int64) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("If-Match", fmt.Sprintf("%d", ifMatch))
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{TenantID: wsID, UserID: userID}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ---------- seed helpers ----------

// qhSeedUser inserts an app_user and returns its id.
func qhSeedUser(t *testing.T, db *sql.DB, ws string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(
		`INSERT INTO app_user (workspace_id, email, display_name) VALUES ($1,$2,'QHTUser') RETURNING id`,
		ws, "qht-"+pgtest.Uniq()+"@example.com",
	).Scan(&id); err != nil {
		t.Fatalf("qhSeedUser: %v", err)
	}
	return id
}

// qhSeedTeam inserts a team and returns its id.
func qhSeedTeam(t *testing.T, db *sql.DB, ws string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(
		`INSERT INTO team (workspace_id, name) VALUES ($1,$2) RETURNING id`,
		ws, "qht-team-"+pgtest.Uniq(),
	).Scan(&id); err != nil {
		t.Fatalf("qhSeedTeam: %v", err)
	}
	return id
}

// qhSeedPipelineStage inserts a pipeline + open stage and returns their ids.
// Uses semantic='open' (win_probability left NULL) so the stage_terminal_prob
// check constraint is not triggered; attainment queries deal.status='won' directly.
func qhSeedPipelineStage(t *testing.T, db *sql.DB, ws string) (pipelineID, stageID string) {
	t.Helper()
	if err := db.QueryRow(
		`INSERT INTO pipeline (workspace_id, name, is_default) VALUES ($1,'QHTPipe',true) RETURNING id`,
		ws,
	).Scan(&pipelineID); err != nil {
		t.Fatalf("qhSeedPipeline: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO stage (workspace_id, pipeline_id, name, position, semantic) VALUES ($1,$2,'Open',1,'open') RETURNING id`,
		ws, pipelineID,
	).Scan(&stageID); err != nil {
		t.Fatalf("qhSeedStage: %v", err)
	}
	return pipelineID, stageID
}

// qhSeedWonDeal inserts a won deal. amount_minor_base is GENERATED as round(amount_minor * fx_rate_to_base).
func qhSeedWonDeal(t *testing.T, db *sql.DB, ws, pipelineID, stageID, ownerID string, amountMinor int64, currency, fxRate string, closedAt time.Time) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, owner_id,
		                    amount_minor, currency, fx_rate_to_base, status, closed_at, source, captured_by)
		 VALUES ($1,'QHTWonDeal',$2,$3,$4,$5,$6,$7,'won',$8,'api','human:qht')`,
		ws, pipelineID, stageID, ownerID, amountMinor, currency, fxRate, closedAt,
	); err != nil {
		t.Fatalf("qhSeedWonDeal: %v", err)
	}
}

// qhSeedNoPermUser inserts a user with a role that has deal.read but no quota permissions.
func qhSeedNoPermUser(t *testing.T, db *sql.DB, ws string) string {
	t.Helper()
	var userID string
	if err := db.QueryRow(
		`INSERT INTO app_user (workspace_id, email, display_name) VALUES ($1,$2,'QHTNoPerm') RETURNING id`,
		ws, "qht-noperm-"+pgtest.Uniq()+"@example.com",
	).Scan(&userID); err != nil {
		t.Fatalf("qhSeedNoPermUser user: %v", err)
	}
	var roleID string
	if err := db.QueryRow(
		`INSERT INTO role (workspace_id, key, is_system, permissions) VALUES ($1,$2,false,'{"deal":{"read":{"row_scope":"all"}}}'::jsonb) RETURNING id`,
		ws, "qht-noperm-"+pgtest.Uniq(),
	).Scan(&roleID); err != nil {
		t.Fatalf("qhSeedNoPermUser role: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO role_assignment (workspace_id, role_id, user_id) VALUES ($1,$2,$3)`,
		ws, roleID, userID,
	); err != nil {
		t.Fatalf("qhSeedNoPermUser role_assignment: %v", err)
	}
	return userID
}

// ---------- decode helpers ----------

type qhQuotaWire struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	OwnerID     *string `json:"owner_id"`
	TeamID      *string `json:"team_id"`
	TargetMinor int64   `json:"target_minor"`
	Currency    string  `json:"currency"`
	Version     int64   `json:"version"`
	ArchivedAt  *string `json:"archived_at"`
}

type qhListWire struct {
	Data []qhQuotaWire `json:"data"`
	Page struct {
		NextCursor any  `json:"next_cursor"`
		HasMore    bool `json:"has_more"`
	} `json:"page"`
}

type qhAttainmentWire struct {
	QuotaID        string  `json:"quota_id"`
	ClosedWonMinor int64   `json:"closed_won_minor"`
	TargetMinor    int64   `json:"target_minor"`
	AttainmentPct  float64 `json:"attainment_pct"`
	GapMinor       int64   `json:"gap_minor"`
	Band           string  `json:"band"`
	ContribDeals   []struct {
		DealID         string `json:"deal_id"`
		BaseValueMinor int64  `json:"base_value_minor"`
	} `json:"contributing_deals"`
}

type qhProblemWire struct {
	Code    string `json:"code"`
	Details struct {
		Errors []struct {
			Field string `json:"field"`
			Code  string `json:"code"`
		} `json:"errors"`
	} `json:"details"`
}

func qhDecodeQuota(t *testing.T, rec *httptest.ResponseRecorder) qhQuotaWire {
	t.Helper()
	var q qhQuotaWire
	if err := json.Unmarshal(rec.Body.Bytes(), &q); err != nil {
		t.Fatalf("decode quota: %v (body=%s)", err, rec.Body)
	}
	return q
}

func qhDecodeList(t *testing.T, rec *httptest.ResponseRecorder) qhListWire {
	t.Helper()
	var l qhListWire
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatalf("decode list: %v (body=%s)", err, rec.Body)
	}
	return l
}

func qhDecodeProblem(t *testing.T, rec *httptest.ResponseRecorder) qhProblemWire {
	t.Helper()
	var p qhProblemWire
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v (body=%s)", err, rec.Body)
	}
	return p
}

// ---------- tests ----------

// TestQuotaHTTP_Create covers POST /quotas: owner-only 201, team-only 201,
// both-set 422 with the exact owner_xor_team_required shape, neither-set 422.
func TestQuotaHTTP_Create(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	h := qhHandler(db)
	userID := qhSeedUser(t, db, ws)
	teamID := qhSeedTeam(t, db, ws)

	t.Run("owner-only 201", func(t *testing.T) {
		body := fmt.Sprintf(`{"owner_id":%q,"period_start":"2025-01-01","period_end":"2025-12-31","target_minor":10000000,"currency":"EUR"}`, userID)
		rec := qhDo(t, h, http.MethodPost, "/quotas", body, ws, userID)
		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201 got %d: %s", rec.Code, rec.Body)
		}
		q := qhDecodeQuota(t, rec)
		if q.ID == "" {
			t.Error("id is empty")
		}
		if q.OwnerID == nil || *q.OwnerID != userID {
			t.Errorf("owner_id = %v, want %s", q.OwnerID, userID)
		}
		if q.TeamID != nil {
			t.Errorf("team_id = %v, want nil", q.TeamID)
		}
	})

	t.Run("team-only 201", func(t *testing.T) {
		body := fmt.Sprintf(`{"team_id":%q,"period_start":"2025-01-01","period_end":"2025-12-31","target_minor":5000000,"currency":"USD"}`, teamID)
		rec := qhDo(t, h, http.MethodPost, "/quotas", body, ws, userID)
		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201 got %d: %s", rec.Code, rec.Body)
		}
		q := qhDecodeQuota(t, rec)
		if q.TeamID == nil || *q.TeamID != teamID {
			t.Errorf("team_id = %v, want %s", q.TeamID, teamID)
		}
		if q.OwnerID != nil {
			t.Errorf("owner_id = %v, want nil", q.OwnerID)
		}
	})

	t.Run("both-set 422 with field error", func(t *testing.T) {
		body := fmt.Sprintf(`{"owner_id":%q,"team_id":%q,"period_start":"2025-01-01","period_end":"2025-12-31","target_minor":1,"currency":"EUR"}`, userID, teamID)
		rec := qhDo(t, h, http.MethodPost, "/quotas", body, ws, userID)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422 got %d: %s", rec.Code, rec.Body)
		}
		p := qhDecodeProblem(t, rec)
		if len(p.Details.Errors) == 0 {
			t.Fatal("want field errors in details.errors")
		}
		fe := p.Details.Errors[0]
		if fe.Field != "owner_id" || fe.Code != "owner_xor_team_required" {
			t.Errorf("field error = %+v, want {owner_id, owner_xor_team_required}", fe)
		}
	})

	t.Run("neither-set 422", func(t *testing.T) {
		body := `{"period_start":"2025-01-01","period_end":"2025-12-31","target_minor":1,"currency":"EUR"}`
		rec := qhDo(t, h, http.MethodPost, "/quotas", body, ws, userID)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422 got %d: %s", rec.Code, rec.Body)
		}
		p := qhDecodeProblem(t, rec)
		if len(p.Details.Errors) == 0 {
			t.Fatal("want field errors in details.errors")
		}
		if p.Details.Errors[0].Code != "owner_xor_team_required" {
			t.Errorf("code = %q, want owner_xor_team_required", p.Details.Errors[0].Code)
		}
	})
}

// TestQuotaHTTP_GetAndList covers GET /quotas/{id} (200, 404) and GET /quotas
// with cursor pagination, owner/team filters, and include_archived.
func TestQuotaHTTP_GetAndList(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	h := qhHandler(db)
	userID := qhSeedUser(t, db, ws)
	teamID := qhSeedTeam(t, db, ws)

	// Create 3 owner-scoped quotas.
	var createdIDs []string
	for i := 0; i < 3; i++ {
		body := fmt.Sprintf(`{"owner_id":%q,"period_start":"2025-01-01","period_end":"2025-12-31","target_minor":%d,"currency":"EUR"}`, userID, (i+1)*1000)
		rec := qhDo(t, h, http.MethodPost, "/quotas", body, ws, userID)
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed quota %d: want 201 got %d", i, rec.Code)
		}
		createdIDs = append(createdIDs, qhDecodeQuota(t, rec).ID)
	}
	// Create 1 team-scoped quota.
	teamBody := fmt.Sprintf(`{"team_id":%q,"period_start":"2025-01-01","period_end":"2025-12-31","target_minor":9999,"currency":"EUR"}`, teamID)
	teamRec := qhDo(t, h, http.MethodPost, "/quotas", teamBody, ws, userID)
	if teamRec.Code != http.StatusCreated {
		t.Fatalf("seed team quota: want 201 got %d", teamRec.Code)
	}

	t.Run("GET /quotas/{id} 200 round-trip", func(t *testing.T) {
		rec := qhDo(t, h, http.MethodGet, "/quotas/"+createdIDs[0], "", ws, userID)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200 got %d: %s", rec.Code, rec.Body)
		}
		q := qhDecodeQuota(t, rec)
		if q.ID != createdIDs[0] {
			t.Errorf("id = %s, want %s", q.ID, createdIDs[0])
		}
		if q.WorkspaceID != ws {
			t.Errorf("workspace_id mismatch")
		}
		if q.TargetMinor != 1000 {
			t.Errorf("target_minor = %d, want 1000", q.TargetMinor)
		}
	})

	t.Run("GET /quotas/{id} 404 nonexistent", func(t *testing.T) {
		rec := qhDo(t, h, http.MethodGet, "/quotas/00000000-0000-0000-0000-000000000000", "", ws, userID)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404 got %d", rec.Code)
		}
		p := qhDecodeProblem(t, rec)
		if p.Code != "not_found" {
			t.Errorf("code = %q, want not_found", p.Code)
		}
	})

	t.Run("GET /quotas default list", func(t *testing.T) {
		rec := qhDo(t, h, http.MethodGet, "/quotas", "", ws, userID)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200 got %d: %s", rec.Code, rec.Body)
		}
		l := qhDecodeList(t, rec)
		if len(l.Data) != 4 {
			t.Errorf("len(data) = %d, want 4 (3 owner + 1 team)", len(l.Data))
		}
	})

	t.Run("GET /quotas filter by owner_id", func(t *testing.T) {
		rec := qhDo(t, h, http.MethodGet, "/quotas?owner_id="+userID, "", ws, userID)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200 got %d", rec.Code)
		}
		l := qhDecodeList(t, rec)
		if len(l.Data) != 3 {
			t.Errorf("owner filter len = %d, want 3", len(l.Data))
		}
	})

	t.Run("GET /quotas filter by team_id", func(t *testing.T) {
		rec := qhDo(t, h, http.MethodGet, "/quotas?team_id="+teamID, "", ws, userID)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200 got %d", rec.Code)
		}
		l := qhDecodeList(t, rec)
		if len(l.Data) != 1 {
			t.Errorf("team filter len = %d, want 1", len(l.Data))
		}
	})

	t.Run("GET /quotas cursor pagination", func(t *testing.T) {
		rec1 := qhDo(t, h, http.MethodGet, "/quotas?limit=2", "", ws, userID)
		if rec1.Code != http.StatusOK {
			t.Fatalf("page1: want 200 got %d", rec1.Code)
		}
		l1 := qhDecodeList(t, rec1)
		if len(l1.Data) != 2 {
			t.Errorf("page1 len = %d, want 2", len(l1.Data))
		}
		if !l1.Page.HasMore {
			t.Error("page1 has_more should be true")
		}
		if l1.Page.NextCursor == nil {
			t.Fatal("page1 next_cursor is nil")
		}
		cursor := fmt.Sprintf("%v", l1.Page.NextCursor)
		rec2 := qhDo(t, h, http.MethodGet, "/quotas?limit=2&cursor="+cursor, "", ws, userID)
		if rec2.Code != http.StatusOK {
			t.Fatalf("page2: want 200 got %d", rec2.Code)
		}
		l2 := qhDecodeList(t, rec2)
		if len(l2.Data) != 2 {
			t.Errorf("page2 len = %d, want 2", len(l2.Data))
		}
	})

	t.Run("GET /quotas include_archived", func(t *testing.T) {
		// Archive one quota.
		delRec := qhDo(t, h, http.MethodDelete, "/quotas/"+createdIDs[0], "", ws, userID)
		if delRec.Code != http.StatusOK {
			t.Fatalf("archive: want 200 got %d", delRec.Code)
		}
		// Default list excludes it.
		liveRec := qhDo(t, h, http.MethodGet, "/quotas", "", ws, userID)
		liveList := qhDecodeList(t, liveRec)
		if len(liveList.Data) != 3 {
			t.Errorf("after archive default list len = %d, want 3", len(liveList.Data))
		}
		// include_archived shows all 4.
		allRec := qhDo(t, h, http.MethodGet, "/quotas?include_archived=true", "", ws, userID)
		allList := qhDecodeList(t, allRec)
		if len(allList.Data) != 4 {
			t.Errorf("include_archived list len = %d, want 4", len(allList.Data))
		}
	})
}

// TestQuotaHTTP_Update covers PATCH /quotas/{id}: valid If-Match (200), stale If-Match (409),
// and the post-merge-patch owner-XOR-team validation (422).
func TestQuotaHTTP_Update(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	h := qhHandler(db)
	userID := qhSeedUser(t, db, ws)
	teamID := qhSeedTeam(t, db, ws)

	// Seed a quota to operate on.
	createBody := fmt.Sprintf(`{"owner_id":%q,"period_start":"2025-01-01","period_end":"2025-12-31","target_minor":10000000,"currency":"EUR"}`, userID)
	createRec := qhDo(t, h, http.MethodPost, "/quotas", createBody, ws, userID)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("seed quota: want 201 got %d", createRec.Code)
	}
	q := qhDecodeQuota(t, createRec)

	t.Run("valid If-Match succeeds 200", func(t *testing.T) {
		patchBody := `{"target_minor":20000000}`
		rec := qhDoIfMatch(t, h, http.MethodPatch, "/quotas/"+q.ID, patchBody, ws, userID, q.Version)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200 got %d: %s", rec.Code, rec.Body)
		}
		updated := qhDecodeQuota(t, rec)
		if updated.TargetMinor != 20000000 {
			t.Errorf("target_minor = %d, want 20000000", updated.TargetMinor)
		}
		if updated.Version <= q.Version {
			t.Errorf("version did not increment: old=%d new=%d", q.Version, updated.Version)
		}
	})

	t.Run("stale If-Match returns 409 version_skew", func(t *testing.T) {
		patchBody := `{"target_minor":999}`
		rec := qhDoIfMatch(t, h, http.MethodPatch, "/quotas/"+q.ID, patchBody, ws, userID, q.Version+100)
		if rec.Code != http.StatusConflict {
			t.Fatalf("want 409 got %d: %s", rec.Code, rec.Body)
		}
		p := qhDecodeProblem(t, rec)
		if p.Code != "version_skew" {
			t.Errorf("code = %q, want version_skew", p.Code)
		}
	})

	t.Run("post-merge owner-XOR-team validation 422", func(t *testing.T) {
		// Quota was created owner-scoped. Patch team_id without clearing owner_id
		// → merged state has both set → 422.
		patchBody := fmt.Sprintf(`{"team_id":%q}`, teamID)
		rec := qhDo(t, h, http.MethodPatch, "/quotas/"+q.ID, patchBody, ws, userID)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422 got %d: %s", rec.Code, rec.Body)
		}
		p := qhDecodeProblem(t, rec)
		if len(p.Details.Errors) == 0 || p.Details.Errors[0].Code != "owner_xor_team_required" {
			t.Errorf("problem = %+v, want owner_xor_team_required", p)
		}
	})
}

// TestQuotaHTTP_Archive covers DELETE /quotas/{id}: returns 200 with the full archived entity
// (never 204); archived_at is set; a subsequent GET returns 404.
func TestQuotaHTTP_Archive(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	h := qhHandler(db)
	userID := qhSeedUser(t, db, ws)

	createBody := fmt.Sprintf(`{"owner_id":%q,"period_start":"2025-01-01","period_end":"2025-12-31","target_minor":1000,"currency":"EUR"}`, userID)
	createRec := qhDo(t, h, http.MethodPost, "/quotas", createBody, ws, userID)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("seed: want 201 got %d", createRec.Code)
	}
	id := qhDecodeQuota(t, createRec).ID

	delRec := qhDo(t, h, http.MethodDelete, "/quotas/"+id, "", ws, userID)
	if delRec.Code != http.StatusOK {
		t.Fatalf("DELETE: want 200 got %d: %s", delRec.Code, delRec.Body)
	}
	archived := qhDecodeQuota(t, delRec)
	if archived.ArchivedAt == nil {
		t.Error("archived_at is nil after DELETE")
	}

	// Subsequent GET must return 404 (archived row is excluded by default Get).
	getAfter := qhDo(t, h, http.MethodGet, "/quotas/"+id, "", ws, userID)
	if getAfter.Code != http.StatusNotFound {
		t.Errorf("GET after archive: want 404 got %d", getAfter.Code)
	}
}

// TestQuotaHTTP_Attainment covers GET /quotas/{id}/attainment end-to-end over real HTTP:
// the golden-number reconciliation, 422 attainment_target_zero, 422 attainment_computation_failed,
// and 404 for a nonexistent quota.
func TestQuotaHTTP_Attainment(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	h := qhHandler(db)

	userID := qhSeedUser(t, db, ws)
	pipelineID, stageID := qhSeedPipelineStage(t, db, ws)

	inPeriod := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	t.Run("golden-number reconciliation (RD-AC-3)", func(t *testing.T) {
		// Quota: 280,000.00 EUR target. Two EUR won deals totaling 313,872.00 EUR.
		body := fmt.Sprintf(`{"owner_id":%q,"period_start":"2024-01-01","period_end":"2024-12-31","target_minor":28000000,"currency":"EUR"}`, userID)
		createRec := qhDo(t, h, http.MethodPost, "/quotas", body, ws, userID)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("seed quota: %d %s", createRec.Code, createRec.Body)
		}
		qID := qhDecodeQuota(t, createRec).ID

		// fx_rate_to_base=1.0 → amount_minor_base = amount_minor (GENERATED column identity).
		qhSeedWonDeal(t, db, ws, pipelineID, stageID, userID, 18000000, "EUR", "1.0000000000", inPeriod)
		qhSeedWonDeal(t, db, ws, pipelineID, stageID, userID, 13387200, "EUR", "1.0000000000", inPeriod)

		rec := qhDo(t, h, http.MethodGet, "/quotas/"+qID+"/attainment", "", ws, userID)
		if rec.Code != http.StatusOK {
			t.Fatalf("attainment: want 200 got %d: %s", rec.Code, rec.Body)
		}
		var att qhAttainmentWire
		if err := json.Unmarshal(rec.Body.Bytes(), &att); err != nil {
			t.Fatalf("decode attainment: %v (body=%s)", err, rec.Body)
		}

		if att.QuotaID != qID {
			t.Errorf("quota_id = %q, want %q", att.QuotaID, qID)
		}
		if att.ClosedWonMinor != 31387200 {
			t.Errorf("closed_won_minor = %d, want 31387200", att.ClosedWonMinor)
		}
		if att.GapMinor != 3387200 {
			t.Errorf("gap_minor = %d, want +3387200 (+33,872.00 EUR)", att.GapMinor)
		}
		wantPct := 31387200.0 / 28000000.0 * 100
		if diff := att.AttainmentPct - wantPct; diff < -0.01 || diff > 0.01 {
			t.Errorf("attainment_pct = %v, want ≈%v", att.AttainmentPct, wantPct)
		}
		if att.Band != "met" {
			t.Errorf("band = %q, want met", att.Band)
		}
		if len(att.ContribDeals) != 2 {
			t.Errorf("contributing_deals len = %d, want 2", len(att.ContribDeals))
		}
		var sumContrib int64
		for _, d := range att.ContribDeals {
			sumContrib += d.BaseValueMinor
		}
		if sumContrib != att.ClosedWonMinor {
			t.Errorf("contributing_deals sum = %d must equal closed_won_minor %d", sumContrib, att.ClosedWonMinor)
		}
	})

	t.Run("target_minor=0 → 422 attainment_target_zero", func(t *testing.T) {
		// target_minor=0 is valid in the DB but refused by Attainment before any deal query.
		body := fmt.Sprintf(`{"owner_id":%q,"period_start":"2024-01-01","period_end":"2024-12-31","target_minor":0,"currency":"EUR"}`, userID)
		createRec := qhDo(t, h, http.MethodPost, "/quotas", body, ws, userID)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("seed zero-target quota: %d %s", createRec.Code, createRec.Body)
		}
		qID := qhDecodeQuota(t, createRec).ID

		rec := qhDo(t, h, http.MethodGet, "/quotas/"+qID+"/attainment", "", ws, userID)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422 got %d: %s", rec.Code, rec.Body)
		}
		p := qhDecodeProblem(t, rec)
		if p.Code != "attainment_target_zero" {
			t.Errorf("code = %q, want attainment_target_zero", p.Code)
		}
	})

	t.Run("missing FX rate → 422 attainment_computation_failed", func(t *testing.T) {
		// USD quota, no USD→EUR fx_rate seeded → FX lookup fails → 422.
		body := fmt.Sprintf(`{"owner_id":%q,"period_start":"2024-01-01","period_end":"2024-12-31","target_minor":10000000,"currency":"USD"}`, userID)
		createRec := qhDo(t, h, http.MethodPost, "/quotas", body, ws, userID)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("seed USD quota: %d %s", createRec.Code, createRec.Body)
		}
		qID := qhDecodeQuota(t, createRec).ID

		rec := qhDo(t, h, http.MethodGet, "/quotas/"+qID+"/attainment", "", ws, userID)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422 got %d: %s", rec.Code, rec.Body)
		}
		p := qhDecodeProblem(t, rec)
		if p.Code != "attainment_computation_failed" {
			t.Errorf("code = %q, want attainment_computation_failed", p.Code)
		}
	})

	t.Run("nonexistent quota → 404", func(t *testing.T) {
		rec := qhDo(t, h, http.MethodGet, "/quotas/00000000-0000-0000-0000-000000000000/attainment", "", ws, userID)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404 got %d: %s", rec.Code, rec.Body)
		}
		p := qhDecodeProblem(t, rec)
		if p.Code != "not_found" {
			t.Errorf("code = %q, want not_found", p.Code)
		}
	})
}

// TestQuotaHTTP_Auth confirms the pre-existing RbacMiddleware fires correctly:
// 401 with no session, 403 for a role with no quota.read permission.
func TestQuotaHTTP_Auth(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)

	noPermUser := qhSeedNoPermUser(t, db, ws)

	// Wrap with RbacMiddleware to engage the auth gates.
	h := platformauth.RbacMiddleware(db, platformauth.ObjQuota)(qhHandler(db))

	// 401: no principal in context — RequireAuth fires before permission check.
	req401 := httptest.NewRequest(http.MethodGet, "/quotas", nil)
	rec401 := httptest.NewRecorder()
	h.ServeHTTP(rec401, req401)
	if rec401.Code != http.StatusUnauthorized {
		t.Errorf("no-session: want 401 got %d; body=%s", rec401.Code, rec401.Body)
	}

	// 403: principal exists but role has no quota.read permission.
	req403 := httptest.NewRequest(http.MethodGet, "/quotas", nil)
	req403 = req403.WithContext(crmctx.With(req403.Context(), crmctx.Principal{TenantID: ws, UserID: noPermUser}))
	rec403 := httptest.NewRecorder()
	h.ServeHTTP(rec403, req403)
	if rec403.Code != http.StatusForbidden {
		t.Errorf("no-quota-perm: want 403 got %d; body=%s", rec403.Code, rec403.Body)
	}
}

// TestQuotaHTTP_RBACBootstrap proves the OP-T04-precedent gap is closed:
// a fresh workspace's bootstrap admin (created via POST /workspaces with adminPermissionsJSON)
// reaches GET /quotas without a 403.
func TestQuotaHTTP_RBACBootstrap(t *testing.T) {
	db := pgtest.OpenTestDB(t)

	// Invoke the real workspace-bootstrap handler (creates workspace + admin role via adminPermissionsJSON).
	bootstrapH := identitytransport.HandleCreateWorkspace(db)
	slug := "rbac-boot-" + pgtest.Uniq()
	reqBody := fmt.Sprintf(`{
		"name":"RBACBoot","slug":%q,"base_currency":"EUR",
		"admin_email":%q,"admin_password":"testPass123","admin_display_name":"Bootstrap Admin"
	}`, slug, slug+"@example.com")
	req := httptest.NewRequest(http.MethodPost, "/workspaces", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	bootstrapH.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /workspaces: want 201 got %d: %s", rec.Code, rec.Body)
	}

	// Parse the returned workspace id.
	var wsResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &wsResp); err != nil {
		t.Fatalf("decode workspace response: %v", err)
	}
	wsID := wsResp.ID

	// Scope the connection's RLS to the new workspace so that LoadRolePermissions
	// (called from RbacMiddleware) can see the role_assignment rows the bootstrap
	// handler just inserted (which have workspace_id = wsID).
	pgtest.SetRLS(t, db, wsID)

	// Retrieve the bootstrap admin's user id from the DB.
	var adminUserID string
	if err := db.QueryRow(
		`SELECT id FROM app_user WHERE workspace_id=$1::uuid LIMIT 1`, wsID,
	).Scan(&adminUserID); err != nil {
		t.Fatalf("get admin user: %v", err)
	}

	// GET /quotas via the RBAC-wrapped handler as the bootstrap admin must NOT return 403.
	h := platformauth.RbacMiddleware(db, platformauth.ObjQuota)(qhHandler(db))
	quotaReq := httptest.NewRequest(http.MethodGet, "/quotas", nil)
	quotaReq = quotaReq.WithContext(crmctx.With(quotaReq.Context(), crmctx.Principal{TenantID: wsID, UserID: adminUserID}))
	quotaRec := httptest.NewRecorder()
	h.ServeHTTP(quotaRec, quotaReq)
	if quotaRec.Code == http.StatusForbidden {
		t.Fatalf("RBAC bootstrap gap: bootstrap admin got 403 on GET /quotas — adminPermissionsJSON missing quota entry")
	}
	if quotaRec.Code == http.StatusUnauthorized {
		t.Fatalf("RBAC bootstrap: got 401 — principal injection failed")
	}
	if quotaRec.Code != http.StatusOK {
		t.Fatalf("GET /quotas as bootstrap admin: want 200 got %d: %s", quotaRec.Code, quotaRec.Body)
	}
}
