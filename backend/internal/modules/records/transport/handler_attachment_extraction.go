package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
	"github.com/gradionhq/margince/backend/internal/shared/ports/extraction"
)

func (h *AttachmentHandler) serveSuffixRoutes(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/extraction"):
		h.getExtraction(w, r, httpkit.PathID(strings.TrimSuffix(r.URL.Path, "/extraction"), "/attachments"))
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/extraction:accept"):
		h.acceptExtraction(w, r, httpkit.PathID(strings.TrimSuffix(r.URL.Path, "/extraction:accept"), "/attachments"))
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/request-access"):
		h.requestAccess(w, r, httpkit.PathID(strings.TrimSuffix(r.URL.Path, "/request-access"), "/attachments"))
	default:
		return false
	}
	return true
}

func (h *AttachmentHandler) getExtraction(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	a, err := h.store.GetAny(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}

	fields, err := h.extractFields(r.Context(), a.ID)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, partitionExtraction(fields))
}

// extractFields runs the configured extractor (or a no-op fallback when none
// is wired) against an attachment.
func (h *AttachmentHandler) extractFields(ctx context.Context, attachmentID string) ([]extraction.ExtractedField, error) {
	extractor := h.extractor
	if extractor == nil {
		extractor = extraction.NoOpExtractor{}
	}
	return extractor.Extract(ctx, attachmentID)
}

func (h *AttachmentHandler) acceptExtraction(w http.ResponseWriter, r *http.Request, id string) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	var body types.AcceptExtractionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, "bad_request")
		return
	}

	a, err := h.store.GetAny(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	if a.EntityType != domain.EntityTypeDeal {
		httpkit.JSONProblem(w, http.StatusUnprocessableEntity, "unsupported_entity_type")
		return
	}

	fields, err := h.extractFields(r.Context(), a.ID)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}

	plan, err := buildAcceptedExtractionPlan(r.Context(), body, fields)
	if err != nil {
		if planErr, ok := err.(acceptedExtractionPlanError); ok {
			httpkit.JSONValidationError(w, planErr.detail, planErr.ferrs)
			return
		}
		httpkit.JSONError(w, err)
		return
	}

	if h.dealWriter == nil {
		httpkit.JSONProblem(w, http.StatusInternalServerError, "missing_deal_writer")
		return
	}
	if err := h.dealWriter.UpdateFields(r.Context(), wsID, a.EntityID, plan.updates); err != nil {
		httpkit.JSONError(w, err)
		return
	}
	if h.audit == nil {
		httpkit.JSONProblem(w, http.StatusInternalServerError, "missing_audit_writer")
		return
	}
	if err := h.writeExtractionAcceptAudits(r.Context(), wsID, a.EntityType, a.EntityID, plan.auditCalls); err != nil {
		httpkit.JSONError(w, err)
		return
	}

	dealID, err := uuid.Parse(a.EntityID)
	if err != nil {
		httpkit.JSONProblem(w, http.StatusInternalServerError, "invalid_deal_id")
		return
	}
	httpkit.JSONOK(w, types.AttachmentExtractionAcceptResponse{
		DealId:   openapi_types.UUID(dealID),
		Accepted: plan.accepted,
	})
}

// writeExtractionAcceptAudits records one audit entry per accepted extraction
// field, stopping at the first failure.
func (h *AttachmentHandler) writeExtractionAcceptAudits(ctx context.Context, wsID, entityType, entityID string, calls []acceptedExtractionAuditCall) error {
	for _, call := range calls {
		if err := h.audit.WriteExtractionAcceptAudit(ctx, wsID, entityType, entityID, call.field, call.sourceQuote, call.capturedBy); err != nil {
			return err
		}
	}
	return nil
}

func (h *AttachmentHandler) requestAccess(w http.ResponseWriter, r *http.Request, id string) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	a, err := h.store.GetAny(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	if h.audit == nil {
		httpkit.JSONProblem(w, http.StatusInternalServerError, "missing_audit_writer")
		return
	}
	if err := h.audit.WriteRequestAccessAudit(r.Context(), wsID, a.EntityType, a.EntityID, a.Filename); err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, types.RequestAccessResponse{Requested: true})
}

func humanCapturedBy(principal crmctx.Principal) string {
	if principal.UserID == "" {
		return "human:"
	}
	return principal.UserID
}

func coerceExtractionValue(field, value string) (any, error) {
	switch field {
	case "amount_minor":
		return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	default:
		return value, nil
	}
}

type acceptedExtractionAuditCall struct {
	field       string
	sourceQuote string
	capturedBy  string
}

type acceptedExtractionPlan struct {
	updates    map[string]any
	accepted   []types.AcceptedExtractionField
	auditCalls []acceptedExtractionAuditCall
}

type acceptedExtractionPlanError struct {
	detail string
	ferrs  []httpkit.FieldError
}

func (e acceptedExtractionPlanError) Error() string { return e.detail }

func buildAcceptedExtractionPlan(ctx context.Context, body types.AcceptExtractionRequest, fields []extraction.ExtractedField) (acceptedExtractionPlan, error) {
	grounded := groundedExtractionFields(fields)
	principal, _ := crmctx.From(ctx)
	edits := map[string]any{}
	if body.Edits != nil {
		edits = *body.Edits
	}
	if len(body.FieldKeys) == 0 {
		return acceptedExtractionPlan{}, acceptedExtractionPlanError{
			detail: "field_keys must name grounded extraction fields.",
			ferrs:  []httpkit.FieldError{{Field: "field_keys", Code: fieldRequired}},
		}
	}

	plan := acceptedExtractionPlan{
		updates:    make(map[string]any, len(body.FieldKeys)),
		accepted:   make([]types.AcceptedExtractionField, 0, len(body.FieldKeys)),
		auditCalls: make([]acceptedExtractionAuditCall, 0, len(body.FieldKeys)),
	}
	var invalid []httpkit.FieldError
	for _, key := range body.FieldKeys {
		extracted, ok := grounded[key]
		if !ok {
			invalid = append(invalid, httpkit.FieldError{Field: "field_keys", Code: key})
			continue
		}

		value := extracted.Value
		provenance := types.AcceptedExtractionFieldProvenanceAiExtracted
		capturedBy := "agent:attachment-extractor"
		if edited, ok := edits[key]; ok {
			value = fmt.Sprint(edited)
			provenance = types.AcceptedExtractionFieldProvenanceHuman
			capturedBy = humanCapturedBy(principal)
		}

		coerced, err := coerceExtractionValue(key, value)
		if err != nil {
			return acceptedExtractionPlan{}, acceptedExtractionPlanError{
				detail: "accepted extraction value could not be converted for one or more fields.",
				ferrs:  []httpkit.FieldError{{Field: key, Code: fieldRequired}},
			}
		}
		plan.updates[key] = coerced
		plan.accepted = append(plan.accepted, types.AcceptedExtractionField{
			Field:      key,
			Value:      value,
			Provenance: provenance,
		})
		plan.auditCalls = append(plan.auditCalls, acceptedExtractionAuditCall{
			field:       key,
			sourceQuote: extracted.SourceQuote,
			capturedBy:  capturedBy,
		})
	}
	if len(invalid) > 0 {
		return acceptedExtractionPlan{}, acceptedExtractionPlanError{
			detail: "field_keys must name grounded extraction fields.",
			ferrs:  invalid,
		}
	}
	return plan, nil
}

func groundedExtractionFields(fields []extraction.ExtractedField) map[string]extraction.ExtractedField {
	grounded := make(map[string]extraction.ExtractedField, len(fields))
	for _, field := range fields {
		if field.Omitted {
			continue
		}
		grounded[field.Field] = field
	}
	return grounded
}

func partitionExtraction(fields []extraction.ExtractedField) types.AttachmentExtraction {
	resp := types.AttachmentExtraction{
		Fields:  make([]types.ExtractedField, 0, len(fields)),
		Omitted: make([]types.OmittedExtractionField, 0),
	}
	for _, field := range fields {
		if field.Omitted {
			resp.Omitted = append(resp.Omitted, types.OmittedExtractionField{
				Field:  field.Field,
				Reason: types.OmittedExtractionFieldReason(field.OmittedReason),
			})
			continue
		}
		resp.Fields = append(resp.Fields, types.ExtractedField{
			Field:         field.Field,
			Value:         field.Value,
			SourceQuote:   field.SourceQuote,
			PageOrSection: field.PageOrSection,
			Confidence:    types.ExtractedFieldConfidence(field.Confidence),
		})
	}
	return resp
}
