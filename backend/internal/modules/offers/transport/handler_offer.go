package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/platform/blobstore"
	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
	"github.com/gradionhq/margince/backend/internal/shared/ports/retrieval"
)

// pathPrefixOffers and pathSegmentLineItems are the two path literals this
// handler's suffix-routing repeatedly matches/strips (SonarCloud "define a
// constant instead of duplicating this literal" finding) — extracted once
// here rather than repeated inline at every routing call site.
const (
	pathPrefixOffers     = "/offers"
	pathSegmentLineItems = "/line-items"
	pathSuffixRender     = "/render"
	pathSuffixSend       = "/send"
	pathSuffixRegenerate = "/regenerate"
)

type offerStoreSeam interface {
	Create(ctx context.Context, o domain.Offer) (domain.Offer, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Offer, error)
	List(ctx context.Context, workspaceID, dealID, cursor string, limit int, includeArchived bool) ([]domain.Offer, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Offer, error)
	Send(ctx context.Context, id, workspaceID string) (domain.Offer, error)
	Regenerate(ctx context.Context, id, workspaceID string, signals []domain.OfferLineSignal) (domain.Offer, error)
	PrepareRender(ctx context.Context, id, workspaceID string) (adapters.RenderIngredients, error)
	SetPdfAssetRef(ctx context.Context, id, workspaceID, ref string) (domain.Offer, error)
}

type offerLineItemStoreSeam interface {
	Create(ctx context.Context, li domain.OfferLineItem, explicitTaxRate *float64) (domain.OfferLineItem, error)
	List(ctx context.Context, offerID, workspaceID string) ([]domain.OfferLineItem, error)
	Update(ctx context.Context, id, offerID, workspaceID string, updates map[string]any) (domain.OfferLineItem, error)
	Delete(ctx context.Context, id, offerID, workspaceID string) error
}

var sendOfferTool = mcp.GeneratedTool{OperationID: "sendOffer", Verb: "send_offer", RecordType: "offer", Tier: mcp.TierYellow}

// OfferHandler routes /deals/{id}/offers, /offers/{id}, /offers/{id}/line-items,
// /offers/{id}/render, /offers/{id}/send, and /offers/{id}/regenerate requests
// (OFFER-WIRE-3/4/5/6/7/8). Mirrors DealHandler's suffix-routing shape — one
// handler, several path shapes, because line items are a draft-only child
// collection of one offer and the action routes stay on the same handler.
type OfferHandler struct {
	offers    offerStoreSeam
	lineItems offerLineItemStoreSeam
	verifier  approvalsport.Verifier
	blob      blobstore.Store
	retriever retrieval.Retriever
}

// NewOfferHandler returns an OfferHandler backed by the given stores.
func NewOfferHandler(offers offerStoreSeam, lineItems offerLineItemStoreSeam, verifier approvalsport.Verifier, blob blobstore.Store, retriever retrieval.Retriever) *OfferHandler {
	return &OfferHandler{offers: offers, lineItems: lineItems, verifier: verifier, blob: blob, retriever: retriever}
}

func (h *OfferHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case strings.HasPrefix(path, "/deals/") && strings.HasSuffix(path, pathPrefixOffers):
		dealID := httpkit.PathID(strings.TrimSuffix(path, pathPrefixOffers), "/deals")
		switch r.Method {
		case http.MethodGet:
			h.listForDeal(w, r, dealID)
		case http.MethodPost:
			h.create(w, r, dealID)
		default:
			http.NotFound(w, r)
		}
	case strings.Contains(path, pathSegmentLineItems):
		h.serveLineItems(w, r, path)
	case r.Method == http.MethodPost && strings.HasSuffix(path, pathSuffixRender):
		h.render(w, r, httpkit.PathID(strings.TrimSuffix(path, pathSuffixRender), pathPrefixOffers))
	case r.Method == http.MethodPost && strings.HasSuffix(path, pathSuffixSend):
		h.send(w, r, httpkit.PathID(strings.TrimSuffix(path, pathSuffixSend), pathPrefixOffers))
	case r.Method == http.MethodPost && strings.HasSuffix(path, pathSuffixRegenerate):
		h.regenerate(w, r, httpkit.PathID(strings.TrimSuffix(path, pathSuffixRegenerate), pathPrefixOffers))
	default:
		h.serveOffer(w, r, path)
	}
}

func (h *OfferHandler) serveOffer(w http.ResponseWriter, r *http.Request, path string) {
	id := httpkit.PathID(path, pathPrefixOffers)
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/regenerate") && id != "":
		h.regenerate(w, r, id)
	case r.Method == http.MethodGet && id != "":
		h.get(w, r, id)
	case r.Method == http.MethodPatch && id != "":
		h.update(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (h *OfferHandler) serveLineItems(w http.ResponseWriter, r *http.Request, path string) {
	idx := strings.Index(path, pathSegmentLineItems)
	offerID := httpkit.PathID(path[:idx], pathPrefixOffers)
	rest := strings.TrimPrefix(strings.TrimPrefix(path[idx:], pathSegmentLineItems), "/")
	switch {
	case rest == "" && r.Method == http.MethodGet:
		h.listLineItems(w, r, offerID)
	case rest == "" && r.Method == http.MethodPost:
		h.createLineItem(w, r, offerID)
	case rest != "" && r.Method == http.MethodPatch:
		h.updateLineItem(w, r, offerID, rest)
	case rest != "" && r.Method == http.MethodDelete:
		h.deleteLineItem(w, r, offerID, rest)
	default:
		http.NotFound(w, r)
	}
}

// ---- offer handlers ----

type createOfferBody struct {
	OfferNumber string  `json:"offer_number"`
	Currency    string  `json:"currency"`
	BuyerOrgID  *string `json:"buyer_org_id"`
	ValidUntil  *string `json:"valid_until"`
	IntroText   *string `json:"intro_text"`
	TermsText   *string `json:"terms_text"`
	TemplateID  *string `json:"template_id"`
	Source      string  `json:"source"`
	CapturedBy  string  `json:"captured_by"`
}

func (h *OfferHandler) create(w http.ResponseWriter, r *http.Request, dealID string) {
	wsID := httpkit.WorkspaceID(r)
	var body createOfferBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	var ferrs []httpkit.FieldError
	if body.OfferNumber == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: "offer_number", Code: codeRequired})
	}
	if body.Currency == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: "currency", Code: codeRequired})
	}
	if body.Source == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: fieldSource, Code: codeRequired})
	}
	if body.CapturedBy == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: fieldCapturedBy, Code: codeRequired})
	}
	if len(ferrs) > 0 {
		httpkit.JSONValidationError(w, "offer_number, currency, source and captured_by are required.", ferrs)
		return
	}
	o := domain.NewOffer(dealID, body.OfferNumber, body.Currency, prov.Provenance{Source: body.Source, CapturedBy: body.CapturedBy})
	o.WorkspaceID = wsID
	o.BuyerOrgID = body.BuyerOrgID
	o.IntroText = body.IntroText
	o.TermsText = body.TermsText
	o.TemplateID = body.TemplateID
	if body.ValidUntil != nil {
		t, err := time.Parse("2006-01-02", *body.ValidUntil)
		if err != nil {
			httpkit.JSONProblem(w, http.StatusBadRequest, "bad_valid_until")
			return
		}
		o.ValidUntil = &t
	}

	created, err := h.offers.Create(r.Context(), o)
	if err != nil {
		if errors.Is(err, adapters.ErrDuplicateOfferNumber) {
			httpkit.JSONProblem(w, http.StatusConflict, "offer_number_duplicate")
			return
		}
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONCreatedAt(w, created, "/offers/"+created.ID)
}

func (h *OfferHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	o, err := h.offers.Get(r.Context(), id, httpkit.WorkspaceID(r))
	writeGetResult(w, o, err)
}

func (h *OfferHandler) listForDeal(w http.ResponseWriter, r *http.Request, dealID string) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	items, next, err := h.offers.List(r.Context(), wsID, dealID, q.Get("cursor"), httpkit.QueryLimit(r, 20), q.Get("include_archived") == "true")
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, httpkit.PageResponse(items, next))
}

func (h *OfferHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	ifMatch, malformed := httpkit.ParseIfMatch(r)
	if malformed {
		httpkit.JSONProblem(w, http.StatusBadRequest, "bad_if_match")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	o, err := h.offers.Update(r.Context(), id, wsID, body, ifMatch)
	if errors.Is(err, adapters.ErrOfferNotDraft) {
		httpkit.JSONProblem(w, http.StatusConflict, "offer_not_draft")
		return
	}
	httpkit.WriteUpdateResult(w, o, err)
}

// ---- line-item handlers ----

type createLineItemBody struct {
	Position       int      `json:"position"`
	ProductID      *string  `json:"product_id"`
	Description    string   `json:"description"`
	Unit           *string  `json:"unit"`
	Quantity       float64  `json:"quantity"`
	UnitPriceMinor int64    `json:"unit_price_minor"`
	DiscountPct    *float64 `json:"discount_pct"`
	TaxRate        *float64 `json:"tax_rate"`
	Source         string   `json:"source"`
	CapturedBy     string   `json:"captured_by"`
}

func (h *OfferHandler) createLineItem(w http.ResponseWriter, r *http.Request, offerID string) {
	wsID := httpkit.WorkspaceID(r)
	var body createLineItemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	var ferrs []httpkit.FieldError
	if body.Description == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: "description", Code: codeRequired})
	}
	if body.Quantity <= 0 {
		ferrs = append(ferrs, httpkit.FieldError{Field: "quantity", Code: "must_be_positive"})
	}
	if body.Source == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: fieldSource, Code: codeRequired})
	}
	if body.CapturedBy == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: fieldCapturedBy, Code: codeRequired})
	}
	if len(ferrs) > 0 {
		httpkit.JSONValidationError(w, "description, quantity, source and captured_by are required (quantity must be > 0).", ferrs)
		return
	}
	li := domain.NewOfferLineItem(offerID, body.Position, body.Description, body.Quantity, body.UnitPriceMinor,
		prov.Provenance{Source: body.Source, CapturedBy: body.CapturedBy})
	li.WorkspaceID = wsID
	li.ProductID = body.ProductID
	if body.Unit != nil {
		li.Unit = *body.Unit
	}
	if body.DiscountPct != nil {
		li.DiscountPct = *body.DiscountPct
	}

	created, err := h.lineItems.Create(r.Context(), li, body.TaxRate)
	if err != nil {
		if errors.Is(err, adapters.ErrOfferNotDraft) {
			httpkit.JSONProblem(w, http.StatusConflict, "offer_not_draft")
			return
		}
		var posErr *adapters.ErrDuplicatePosition
		if errors.As(err, &posErr) {
			httpkit.JSONProblemDetails(w, http.StatusConflict, "offer_line_item_position_duplicate",
				"A line item at this position already exists.",
				map[string]any{fieldExistingID: posErr.ExistingID, "field": "position"})
			return
		}
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONCreatedAt(w, created, "/offers/"+offerID+"/line-items/"+created.ID)
}

func (h *OfferHandler) listLineItems(w http.ResponseWriter, r *http.Request, offerID string) {
	items, err := h.lineItems.List(r.Context(), offerID, httpkit.WorkspaceID(r))
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, httpkit.PageResponse(items, ""))
}

func (h *OfferHandler) updateLineItem(w http.ResponseWriter, r *http.Request, offerID, lineID string) {
	wsID := httpkit.WorkspaceID(r)
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	li, err := h.lineItems.Update(r.Context(), lineID, offerID, wsID, body)
	if errors.Is(err, adapters.ErrOfferNotDraft) {
		httpkit.JSONProblem(w, http.StatusConflict, "offer_not_draft")
		return
	}
	var posErr *adapters.ErrDuplicatePosition
	if errors.As(err, &posErr) {
		httpkit.JSONProblemDetails(w, http.StatusConflict, "offer_line_item_position_duplicate",
			"A line item at this position already exists.",
			map[string]any{fieldExistingID: posErr.ExistingID, "field": "position"})
		return
	}
	httpkit.WriteUpdateResult(w, li, err)
}

func (h *OfferHandler) deleteLineItem(w http.ResponseWriter, r *http.Request, offerID, lineID string) {
	err := h.lineItems.Delete(r.Context(), lineID, offerID, httpkit.WorkspaceID(r))
	if errors.Is(err, adapters.ErrOfferNotDraft) {
		httpkit.JSONProblem(w, http.StatusConflict, "offer_not_draft")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- offer actions ----

func (h *OfferHandler) render(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	ingredients, err := h.offers.PrepareRender(r.Context(), id, wsID)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	pdfBytes, err := adapters.RenderOfferPDF(ingredients.Offer, ingredients.LineItems, ingredients.BuyerBlock, ingredients.IssuerName, ingredients.Locale)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	ref, err := h.blob.Put(r.Context(), fmt.Sprintf("offers/%s/%s/%d.pdf", wsID, id, ingredients.Offer.Revision), bytes.NewReader(pdfBytes))
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	updated, err := h.offers.SetPdfAssetRef(r.Context(), id, wsID, ref)
	httpkit.WriteUpdateResult(w, updated, err)
}

func (h *OfferHandler) send(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	p, _ := crmctx.From(r.Context())
	diffFields := map[string]any{"offer_id": id}
	if err := toolgate.Enforce(r.Context(), p, h.verifier, sendOfferTool, wsID, diffFields, nil, r.Header.Get("X-Approval-Token")); err != nil {
		if errors.Is(err, toolgate.ErrApprovalRequired) {
			httpkit.JSONProblem(w, http.StatusForbidden, "approval_required")
		} else {
			httpkit.JSONProblem(w, http.StatusForbidden, "approval_token_invalid")
		}
		return
	}
	updated, err := h.offers.Send(r.Context(), id, wsID)
	if errors.Is(err, adapters.ErrOfferNotDraft) {
		httpkit.JSONProblem(w, http.StatusConflict, "offer_not_draft")
		return
	}
	var fxErr *deals.FXRateUnavailableError
	if errors.As(err, &fxErr) {
		jsonFXRateUnavailable(w, fxErr)
		return
	}
	httpkit.WriteUpdateResult(w, updated, err)
}

// regenerate resolves the offer, assembles the deal's retrieval context and
// decodes it into candidate AI line signals (a no-op today —
// transport.NewNoOpRetriever always yields an empty context, so signals is
// always nil/empty until a real retriever is wired — see
// decodeOfferLineSignals), and hands both id and the decoded signals to
// OfferStore.Regenerate, which always clones the prior sent offer's line
// items verbatim and layers AI-authored lines on top only when grounded
// signals are present (OFFER-AC-10d). The precondition is requireSent, not
// requireDraft: crm.yaml's regenerateOffer operation is documented as
// "regenerate a sent offer into a new draft revision".
func (h *OfferHandler) regenerate(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	offer, err := h.offers.Get(r.Context(), id, wsID)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	assembled, err := h.retriever.AssembleContext(r.Context(), offer.DealID)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	updated, err := h.offers.Regenerate(r.Context(), id, wsID, decodeOfferLineSignals(assembled))
	if errors.Is(err, adapters.ErrOfferNotSent) {
		httpkit.JSONProblem(w, http.StatusConflict, "offer_not_sent")
		return
	}
	httpkit.WriteUpdateResult(w, updated, err)
}

func jsonFXRateUnavailable(w http.ResponseWriter, err *deals.FXRateUnavailableError) {
	httpkit.JSONProblemDetails(w, http.StatusUnprocessableEntity, "fx_rate_unavailable",
		fmt.Sprintf("No stored FX rate for %s as of %s.", err.Currency, err.AsOf.Format("2006-01-02")),
		map[string]any{"currency": err.Currency, "as_of": err.AsOf.Format("2006-01-02")})
}
