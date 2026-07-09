package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
	"github.com/gradionhq/margince/backend/internal/shared/ports/retrieval"
)

// pathPrefixOffers and pathSegmentLineItems are the two path literals this
// handler's suffix-routing repeatedly matches/strips (SonarCloud "define a
// constant instead of duplicating this literal" finding) — extracted once
// here rather than repeated inline at every routing call site.
const (
	pathPrefixOffers     = "/offers"
	pathSegmentLineItems = "/line-items"
)

type offerStoreSeam interface {
	Create(ctx context.Context, o domain.Offer) (domain.Offer, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Offer, error)
	List(ctx context.Context, workspaceID, dealID, cursor string, limit int, includeArchived bool) ([]domain.Offer, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Offer, error)
	Regenerate(ctx context.Context, id, workspaceID string, signals []domain.OfferLineSignal) (domain.Offer, error)
}

type offerLineItemStoreSeam interface {
	Create(ctx context.Context, li domain.OfferLineItem, explicitTaxRate *float64) (domain.OfferLineItem, error)
	List(ctx context.Context, offerID, workspaceID string) ([]domain.OfferLineItem, error)
	Update(ctx context.Context, id, offerID, workspaceID string, updates map[string]any) (domain.OfferLineItem, error)
	Delete(ctx context.Context, id, offerID, workspaceID string) error
}

// OfferHandler routes /deals/{id}/offers, /offers/{id}, /offers/{id}/regenerate,
// /offers/{id}/line-items, and /offers/{id}/line-items/{lineId} requests
// (OFFER-WIRE-3/4/5/6). Mirrors DealHandler's suffix-routing shape — one
// handler, several path shapes, because line items are a draft-only child
// collection of one offer.
type OfferHandler struct {
	offers    offerStoreSeam
	lineItems offerLineItemStoreSeam
	retriever retrieval.Retriever
}

// NewOfferHandler returns an OfferHandler backed by the given stores.
func NewOfferHandler(offers offerStoreSeam, lineItems offerLineItemStoreSeam, retriever retrieval.Retriever) *OfferHandler {
	return &OfferHandler{offers: offers, lineItems: lineItems, retriever: retriever}
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

func (h *OfferHandler) regenerate(w http.ResponseWriter, r *http.Request, id string) {
	assembled, err := h.retriever.AssembleContext(r.Context(), id)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	o, err := h.offers.Regenerate(r.Context(), id, httpkit.WorkspaceID(r), decodeOfferLineSignals(assembled))
	if errors.Is(err, adapters.ErrOfferNotDraft) {
		httpkit.JSONProblem(w, http.StatusConflict, "offer_not_draft")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, o)
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
