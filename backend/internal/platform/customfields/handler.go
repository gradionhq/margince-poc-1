package customfields

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// createCustomFieldTool is the createCustomField x-mcp-tool declaration
// (crm.yaml: create_record/custom_field/yellow — see tools_gen.go's
// generated table). Always yellow, never a dynamic-tier resolver.
var createCustomFieldTool = mcp.GeneratedTool{OperationID: "createCustomField", Verb: "create_record", RecordType: "custom_field", Tier: mcp.TierYellow}

// Handler serves /custom-fields (CF-T03): POST (create) is wired to the
// governed add-field engine; GET (list) stays 501 — CF-T02 contracted it,
// wiring it is a future ticket's job (Out of scope).
type Handler struct {
	db       *sql.DB
	verifier approvalsport.Verifier
}

// NewHandler returns a Handler.
func NewHandler(db *sql.DB, verifier approvalsport.Verifier) *Handler {
	return &Handler{db: db, verifier: verifier}
}

// ServeHTTP dispatches on method.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.create(w, r)
		return
	}
	w.WriteHeader(http.StatusNotImplemented)
}

type createCustomFieldRequest struct {
	Object     string   `json:"object"`
	Label      string   `json:"label"`
	Type       string   `json:"type"`
	Currency   *string  `json:"currency"`
	Options    []string `json:"options"`
	Source     string   `json:"source"`
	CapturedBy string   `json:"captured_by"`
}

// create implements POST /custom-fields (createCustomField, CUSTOM-FIELDS-WIRE-2,
// x-mcp-tool create_record/custom_field/yellow). Mirrors
// handler_person_merge.go's merge()/handler_deal_advance.go's advance(): a
// human principal's direct call is itself the approval (no token, ever); an
// agent principal must present a single-use X-Approval-Token bound to this
// exact (workspace, tool, diff). No column/catalog/audit is written until
// the gate admits (RC-12).
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body createCustomFieldRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, "bad_request", "Request body must be valid JSON.")
		return
	}

	p, _ := crmctx.From(r.Context())
	wsID := p.TenantID
	diffFields := map[string]any{fieldObject: body.Object, fieldLabel: body.Label, fieldType: body.Type}
	if err := toolgate.Enforce(r.Context(), p, h.verifier, createCustomFieldTool, wsID, diffFields, nil, r.Header.Get("X-Approval-Token")); err != nil {
		if errors.Is(err, toolgate.ErrApprovalRequired) {
			jsonProblem(w, http.StatusForbidden, "approval_required", "Adding a custom field is a schema change and is confirm-first; supply an approval token.")
		} else {
			jsonProblem(w, http.StatusForbidden, "approval_token_invalid", "The approval token is missing, expired, already consumed, or does not match this request.")
		}
		return
	}

	spec := FieldSpec{Object: body.Object, Label: body.Label, Type: body.Type, Source: body.Source, CapturedBy: body.CapturedBy}
	if body.Currency != nil {
		spec.Currency = *body.Currency
	}
	if body.Options != nil {
		spec.Options = body.Options
	}

	created, err := Create(r.Context(), h.db, spec)
	var verr *ErrValidation
	switch {
	case errors.As(err, &verr):
		jsonValidationError(w, "One or more fields are invalid.", verr.Errors)
	case errors.Is(err, ErrStructural):
		jsonProblemDetails(w, http.StatusUnprocessableEntity, "structural_change_refused",
			"This looks like a new object, relationship, or logic — not a scalar attribute on an existing object. Runtime custom fields only add bounded scalar columns; a structural change ships as a reviewed source change instead.",
			map[string]any{"route": "source_development_path"})
	case err != nil:
		jsonProblem(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
	default:
		jsonCreated(w, created)
	}
}

// jsonKeyStatus / jsonKeyCode / jsonKeyDetail are the JSON response body's
// "status"/"code"/"detail" keys, shared by the three response writers below
// — extracted to satisfy golangci-lint's goconst rule (each literal repeats
// 3+ times). Named jsonKeyStatus (not status) to avoid shadowing the status
// int parameter these functions already take.
const (
	jsonKeyStatus = "status"
	jsonKeyCode   = "code"
	jsonKeyDetail = "detail"
)

func jsonCreated(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

func jsonProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{jsonKeyStatus: status, jsonKeyCode: code, jsonKeyDetail: detail}) //nolint:errcheck,gosec
}

func jsonProblemDetails(w http.ResponseWriter, status int, code, detail string, details map[string]any) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{jsonKeyStatus: status, jsonKeyCode: code, jsonKeyDetail: detail, "details": details}) //nolint:errcheck,gosec
}

func jsonValidationError(w http.ResponseWriter, detail string, errs []FieldError) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck,gosec
		jsonKeyStatus: http.StatusUnprocessableEntity, jsonKeyCode: "validation_error",
		jsonKeyDetail: detail, "details": map[string]any{"errors": errs},
	})
}
