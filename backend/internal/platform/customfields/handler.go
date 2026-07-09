package customfields

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

const (
	mpcVerbUpdateRecord = "update_record"
	mpcRecordTypeField  = "custom_field"
)

// createCustomFieldTool is the createCustomField x-mcp-tool declaration
// (crm.yaml: create_record/custom_field/yellow — see tools_gen.go's
// generated table). Always yellow, never a dynamic-tier resolver.
var createCustomFieldTool = mcp.GeneratedTool{OperationID: "createCustomField", Verb: "create_record", RecordType: mpcRecordTypeField, Tier: mcp.TierYellow}

// renameCustomFieldTool is the renameCustomField x-mcp-tool declaration
// (crm.yaml: update_record/custom_field/green). Always green — toolgate.Enforce
// passes any agent principal freely, kept for uniformity with the other two tools.
var renameCustomFieldTool = mcp.GeneratedTool{OperationID: "renameCustomField", Verb: mpcVerbUpdateRecord, RecordType: mpcRecordTypeField, Tier: mcp.TierGreen}

// retireCustomFieldTool is the retireCustomField x-mcp-tool declaration
// (crm.yaml: update_record/custom_field/yellow).
var retireCustomFieldTool = mcp.GeneratedTool{OperationID: "retireCustomField", Verb: mpcVerbUpdateRecord, RecordType: mpcRecordTypeField, Tier: mcp.TierYellow}

// updateCustomFieldOptionsTool is the updateCustomFieldOptions x-mcp-tool
// declaration (crm.yaml: update_record/custom_field/yellow, CF-T04).
var updateCustomFieldOptionsTool = mcp.GeneratedTool{OperationID: "updateCustomFieldOptions", Verb: mpcVerbUpdateRecord, RecordType: mpcRecordTypeField, Tier: mcp.TierYellow}

// Handler serves /custom-fields: GET (list) is the admin field-table read
// (listCustomFields, CUSTOM-FIELDS-WIRE-1); POST (create) is wired to the
// governed add-field engine (CF-T03); PATCH (rename) + the /retire and
// /options suffix routes cover the lifecycle (CF-T04).
type Handler struct {
	db       *sql.DB
	verifier approvalsport.Verifier
}

// NewHandler returns a Handler.
func NewHandler(db *sql.DB, verifier approvalsport.Verifier) *Handler {
	return &Handler{db: db, verifier: verifier}
}

// pathID extracts an ID from a request path. Given "/custom-fields/abc" and
// prefix "/custom-fields", returns "abc".
func pathID(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimPrefix(rest, "/")
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// ServeHTTP dispatches on method + path. The collection route (POST create)
// is method-only; the three id-scoped routes need the id (and, for retire/
// options, a path suffix) parsed out of r.URL.Path — this Handler is
// mounted at both "/custom-fields" and "/custom-fields/" by cmd/api/routes.go's
// raw *http.ServeMux registration, mirroring
// organizations/transport/handler_org.go's own suffix-route dispatch.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.serveSuffixRoutes(w, r) {
		return
	}
	id := pathID(r.URL.Path, "/custom-fields")
	switch {
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	case r.Method == http.MethodPost && id == "":
		h.create(w, r)
	case r.Method == http.MethodPatch && id != "":
		h.rename(w, r, id)
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

// serveSuffixRoutes dispatches the /retire and /options suffix routes,
// keeping ServeHTTP's cyclomatic complexity within the T1 lint budget
// (mirrors organizations/transport/handler_org.go's serveSuffixRoutes).
func (h *Handler) serveSuffixRoutes(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/retire") {
		id := pathID(strings.TrimSuffix(r.URL.Path, "/retire"), "/custom-fields")
		h.retire(w, r, id)
		return true
	}
	if r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/options") {
		id := pathID(strings.TrimSuffix(r.URL.Path, "/options"), "/custom-fields")
		h.setOptions(w, r, id)
		return true
	}
	return false
}

// list implements GET /custom-fields?object=<obj>[&status=active|retired]
// (listCustomFields, CUSTOM-FIELDS-WIRE-1, x-mcp-tool
// search_records/custom_field/green — a read, never gated). Backs the
// custom-fields admin field table; returns both active and retired rows by
// default, narrowable by status.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	object := r.URL.Query().Get("object")
	if object == "" {
		jsonProblem(w, http.StatusBadRequest, "bad_request", "The 'object' query parameter is required.")
		return
	}
	status := r.URL.Query().Get("status")

	p, _ := crmctx.From(r.Context())
	fields, err := List(r.Context(), h.db, p.TenantID, object, status)
	if err != nil {
		jsonProblem(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
		return
	}
	jsonOK(w, map[string]any{
		"data": fields,
		"page": map[string]any{"next_cursor": nil, "has_more": false},
	})
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

type renameCustomFieldRequest struct {
	Label string `json:"label"`
}

// rename implements PATCH /custom-fields/{id} (renameCustomField,
// CUSTOM-FIELDS-WIRE-3, x-mcp-tool update_record/custom_field/green). 🟢:
// toolgate.Enforce is called for uniformity with create/retire/options, but
// a green tier always passes — no approval token is ever required.
func (h *Handler) rename(w http.ResponseWriter, r *http.Request, id string) {
	var body renameCustomFieldRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, "bad_request", "Request body must be valid JSON.")
		return
	}

	p, _ := crmctx.From(r.Context())
	if err := toolgate.Enforce(r.Context(), p, h.verifier, renameCustomFieldTool, p.TenantID, map[string]any{fieldLabel: body.Label}, nil, r.Header.Get("X-Approval-Token")); err != nil {
		if errors.Is(err, toolgate.ErrApprovalRequired) {
			jsonProblem(w, http.StatusForbidden, "approval_required", "Approval required.")
		} else {
			jsonProblem(w, http.StatusForbidden, "approval_token_invalid", "The approval token is missing, expired, already consumed, or does not match this request.")
		}
		return
	}

	renamed, err := Rename(r.Context(), h.db, id, body.Label)
	var verr *ErrValidation
	switch {
	case errors.As(err, &verr):
		jsonValidationError(w, "One or more fields are invalid.", verr.Errors)
	case errors.Is(err, ErrNotFound):
		jsonProblem(w, http.StatusNotFound, "not_found", "No custom field exists with this id.")
	case err != nil:
		jsonProblem(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
	default:
		jsonOK(w, renamed)
	}
}

// retire implements POST /custom-fields/{id}/retire (retireCustomField,
// CUSTOM-FIELDS-WIRE-4, x-mcp-tool update_record/custom_field/yellow).
// Mirrors create()'s toolgate shape exactly.
func (h *Handler) retire(w http.ResponseWriter, r *http.Request, id string) {
	p, _ := crmctx.From(r.Context())
	if err := toolgate.Enforce(r.Context(), p, h.verifier, retireCustomFieldTool, p.TenantID, map[string]any{"id": id}, nil, r.Header.Get("X-Approval-Token")); err != nil {
		if errors.Is(err, toolgate.ErrApprovalRequired) {
			jsonProblem(w, http.StatusForbidden, "approval_required", "Retiring a custom field is confirm-first; supply an approval token.")
		} else {
			jsonProblem(w, http.StatusForbidden, "approval_token_invalid", "The approval token is missing, expired, already consumed, or does not match this request.")
		}
		return
	}

	retired, err := Retire(r.Context(), h.db, id)
	switch {
	case errors.Is(err, ErrNotFound):
		jsonProblem(w, http.StatusNotFound, "not_found", "No custom field exists with this id.")
	case err != nil:
		jsonProblem(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
	default:
		jsonOK(w, retired)
	}
}

type setCustomFieldOptionsRequest struct {
	Options []string `json:"options"`
}

// setOptions implements PATCH /custom-fields/{id}/options
// (updateCustomFieldOptions, CUSTOM-FIELDS-PARAM-5, x-mcp-tool
// update_record/custom_field/yellow, CF-T04).
func (h *Handler) setOptions(w http.ResponseWriter, r *http.Request, id string) {
	var body setCustomFieldOptionsRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, "bad_request", "Request body must be valid JSON.")
		return
	}

	p, _ := crmctx.From(r.Context())
	if err := toolgate.Enforce(r.Context(), p, h.verifier, updateCustomFieldOptionsTool, p.TenantID, map[string]any{fieldOptions: body.Options}, nil, r.Header.Get("X-Approval-Token")); err != nil {
		if errors.Is(err, toolgate.ErrApprovalRequired) {
			jsonProblem(w, http.StatusForbidden, "approval_required", "Editing a picklist's options is confirm-first; supply an approval token.")
		} else {
			jsonProblem(w, http.StatusForbidden, "approval_token_invalid", "The approval token is missing, expired, already consumed, or does not match this request.")
		}
		return
	}

	updated, err := SetOptions(r.Context(), h.db, id, body.Options)
	switch {
	case errors.Is(err, ErrLastOption):
		jsonValidationError(w, "A picklist needs at least one option", []FieldError{{Field: "options", Code: "min_one_required"}})
	case errors.Is(err, ErrNotPicklist):
		jsonProblemDetails(w, http.StatusUnprocessableEntity, "not_picklist", "Only a picklist field's options can be edited.", nil)
	case errors.Is(err, ErrNotFound):
		jsonProblem(w, http.StatusNotFound, "not_found", "No custom field exists with this id.")
	case err != nil:
		jsonProblem(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
	default:
		jsonOK(w, updated)
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set(headerContentType, "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

// jsonKeyStatus / jsonKeyCode / jsonKeyDetail are the JSON response body's
// "status"/"code"/"detail" keys, shared by the three response writers below
// — extracted to satisfy golangci-lint's goconst rule (each literal repeats
// 3+ times). Named jsonKeyStatus (not status) to avoid shadowing the status
// int parameter these functions already take.
//
// headerContentType / contentTypeProblemJSON follow the same pattern: the
// "Content-Type" header name and the "application/problem+json" media type
// each repeat across these same response writers.
const (
	jsonKeyStatus = "status"
	jsonKeyCode   = "code"
	jsonKeyDetail = "detail"

	headerContentType      = "Content-Type"
	contentTypeProblemJSON = "application/problem+json"
)

func jsonCreated(w http.ResponseWriter, v any) {
	w.Header().Set(headerContentType, "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

func jsonProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set(headerContentType, contentTypeProblemJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{jsonKeyStatus: status, jsonKeyCode: code, jsonKeyDetail: detail}) //nolint:errcheck,gosec
}

func jsonProblemDetails(w http.ResponseWriter, status int, code, detail string, details map[string]any) {
	w.Header().Set(headerContentType, contentTypeProblemJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{jsonKeyStatus: status, jsonKeyCode: code, jsonKeyDetail: detail, "details": details}) //nolint:errcheck,gosec
}

func jsonValidationError(w http.ResponseWriter, detail string, errs []FieldError) {
	w.Header().Set(headerContentType, contentTypeProblemJSON)
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck,gosec
		jsonKeyStatus: http.StatusUnprocessableEntity, jsonKeyCode: "validation_error",
		jsonKeyDetail: detail, "details": map[string]any{"errors": errs},
	})
}
