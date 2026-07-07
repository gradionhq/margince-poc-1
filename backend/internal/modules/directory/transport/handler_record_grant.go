package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// recordGrantsPath is the collection path for GET/POST /record-grants.
const recordGrantsPath = "/record-grants"

// RecordGrantHandler implements GET/POST /record-grants and
// DELETE /record-grants/{id} (crm.yaml listRecordGrants/createRecordGrant/
// revokeRecordGrant). Both writes are 🟡 (x-mcp-tool share_record) — an agent
// principal must present a valid X-Approval-Token; a human's direct call is
// itself the approval (mirrors handler_person_merge.go's merge / handler_deal.go's
// checkApprovalGate — GH-209 WS-B#3).
type RecordGrantHandler struct {
	store *directory.RecordGrantStore
	db    *sql.DB
}

// NewRecordGrantHandler returns a RecordGrantHandler.
func NewRecordGrantHandler(store *directory.RecordGrantStore, db *sql.DB) *RecordGrantHandler {
	return &RecordGrantHandler{store: store, db: db}
}

func (h *RecordGrantHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == recordGrantsPath:
		h.list(w, r)
	case r.Method == http.MethodPost && r.URL.Path == recordGrantsPath:
		h.create(w, r)
	case r.Method == http.MethodDelete:
		h.revoke(w, r)
	default:
		jsonProblem(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func (h *RecordGrantHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID := workspaceID(r)
	q := r.URL.Query()
	filter := directory.RecordGrantListFilter{
		RecordType:  q.Get("record_type"),
		RecordID:    q.Get("record_id"),
		SubjectType: q.Get("subject_type"),
		SubjectID:   q.Get("subject_id"),
		Cursor:      q.Get("cursor"),
	}
	grants, next, err := h.store.List(r.Context(), wsID, filter, 20)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(grants, next))
}

func (h *RecordGrantHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := workspaceID(r)
	var body struct {
		RecordType  string  `json:"record_type"`
		RecordID    string  `json:"record_id"`
		SubjectType string  `json:"subject_type"`
		SubjectID   string  `json:"subject_id"`
		Access      string  `json:"access"`
		Reason      *string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}

	p, _ := crmctx.From(r.Context())
	if p.IsAgent {
		token := r.Header.Get("X-Approval-Token")
		if token == "" {
			jsonProblem(w, http.StatusForbidden, "approval_required")
			return
		}
		diffHash := crmapprovals.HashDiff(map[string]any{
			"record_type": body.RecordType, "record_id": body.RecordID,
			"subject_type": body.SubjectType, "subject_id": body.SubjectID, "access": body.Access,
		})
		if err := crmapprovals.VerifyAndConsume(r.Context(), h.db, token, crmapprovals.Binding{
			WorkspaceID: wsID, Tool: "share_record", DiffHash: diffHash,
		}); err != nil {
			jsonProblem(w, http.StatusForbidden, "approval_token_invalid")
			return
		}
	}

	grantorAccess, err := h.resolveGrantorOwnAccess(r.Context(), wsID, p.UserID, body.RecordType, body.RecordID)
	if err != nil {
		jsonErr(w, err)
		return
	}

	g, err := h.store.Create(r.Context(), directory.CreateRecordGrantInput{
		WorkspaceID: wsID, RecordType: body.RecordType, RecordID: body.RecordID,
		SubjectType: body.SubjectType, SubjectID: body.SubjectID, Access: body.Access,
		GrantedBy: p.UserID, Reason: body.Reason, GrantorOwnAccess: grantorAccess,
	})
	if errors.Is(err, directory.ErrGrantExceedsGrantorAccess) {
		jsonProblem(w, http.StatusForbidden, "scope_exceeded")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(g) //nolint:errcheck,gosec
}

func (h *RecordGrantHandler) revoke(w http.ResponseWriter, r *http.Request) {
	wsID := workspaceID(r)
	id := pathID(r.URL.Path, recordGrantsPath)

	p, _ := crmctx.From(r.Context())
	if p.IsAgent {
		token := r.Header.Get("X-Approval-Token")
		if token == "" {
			jsonProblem(w, http.StatusForbidden, "approval_required")
			return
		}
		diffHash := crmapprovals.HashDiff(map[string]any{"id": id})
		if err := crmapprovals.VerifyAndConsume(r.Context(), h.db, token, crmapprovals.Binding{
			WorkspaceID: wsID, Tool: "share_record", DiffHash: diffHash,
		}); err != nil {
			jsonProblem(w, http.StatusForbidden, "approval_token_invalid")
			return
		}
	}

	if err := h.store.Revoke(r.Context(), id, wsID); err != nil {
		jsonErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolveGrantorOwnAccess is the scope-intersection approximation described in
// the plan's Task 6 design note (design deviation D2 — no general row-scope
// resolver exists to consult role.permissions.row_scope precisely): "write" if
// the principal owns the record; else the access level of any record_grant
// they themselves already hold on it; else "read" (RBAC/RLS already gated
// that this principal can see the record at all to be requesting a grant on
// it in the first place).
func (h *RecordGrantHandler) resolveGrantorOwnAccess(ctx context.Context, wsID, userID, recordType, recordID string) (string, error) {
	// ownerQuery is a fixed, literal, per-record-type SELECT — never built
	// from interpolated input — covering the only four values
	// record_grant.record_type's CHECK constraint allows (migration 000069).
	var ownerQuery string
	switch recordType {
	case "deal":
		ownerQuery = `SELECT owner_id FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid`
	case "person":
		ownerQuery = `SELECT owner_id FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid`
	case "organization":
		ownerQuery = `SELECT owner_id FROM organization WHERE id=$1::uuid AND workspace_id=$2::uuid`
	case "lead":
		ownerQuery = `SELECT owner_id FROM lead WHERE id=$1::uuid AND workspace_id=$2::uuid`
	default:
		return "", fmt.Errorf("resolveGrantorOwnAccess: unknown record_type %q", recordType)
	}

	var access string
	err := database.WithWorkspaceTx(ctx, h.db, wsID, func(tx *sql.Tx) error {
		var ownerID string
		err := tx.QueryRowContext(ctx, ownerQuery, recordID, wsID).Scan(&ownerID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if ownerID == userID {
			access = "write"
			return nil
		}

		var existing string
		err = tx.QueryRowContext(ctx, `
			SELECT access FROM record_grant
			WHERE workspace_id=$1::uuid AND record_type=$2 AND record_id=$3::uuid
			  AND subject_type='user' AND subject_id=$4::uuid
			  AND (expires_at IS NULL OR expires_at > now())`,
			wsID, recordType, recordID, userID).Scan(&existing)
		switch {
		case err == nil:
			access = existing
		case errors.Is(err, sql.ErrNoRows):
			access = "read"
		default:
			return err
		}
		return nil
	})
	return access, err
}
