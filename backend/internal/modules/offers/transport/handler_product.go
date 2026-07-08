package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

type productStoreSeam interface {
	Create(ctx context.Context, p domain.Product) (domain.Product, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Product, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool) ([]domain.Product, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Product, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Product, error)
}

// ProductHandler routes /products and /products/{id} requests.
type ProductHandler struct{ store productStoreSeam }

// NewProductHandler returns a ProductHandler backed by the given store.
func NewProductHandler(store productStoreSeam) *ProductHandler {
	return &ProductHandler{store: store}
}

func (h *ProductHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dispatchCRUD(w, r, "/products", crudHandlers{list: h.list, create: h.create, get: h.get, update: h.update, archive: h.archive})
}

type createProductBody struct {
	Name           string   `json:"name"`
	SKU            *string  `json:"sku"`
	Description    *string  `json:"description"`
	Unit           *string  `json:"unit"`
	UnitPriceMinor int64    `json:"unit_price_minor"`
	Currency       string   `json:"currency"`
	DefaultTaxRate *float64 `json:"default_tax_rate"`
	Active         *bool    `json:"active"`
	Source         string   `json:"source"`
	CapturedBy     string   `json:"captured_by"`
}

func (h *ProductHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := httpkit.WorkspaceID(r)
	var body createProductBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.Source == "" || body.CapturedBy == "" {
		httpkit.JSONValidationError(w, "source and captured_by are required.",
			[]httpkit.FieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})
		return
	}
	p := domain.NewProduct(body.Name, prov.Provenance{Source: body.Source, CapturedBy: body.CapturedBy})
	p.WorkspaceID = wsID
	p.SKU = body.SKU
	p.Description = body.Description
	if body.Unit != nil {
		p.Unit = body.Unit
	}
	p.UnitPriceMinor = body.UnitPriceMinor
	p.Currency = body.Currency
	p.DefaultTaxRate = body.DefaultTaxRate
	if body.Active != nil {
		p.Active = *body.Active
	}

	created, err := h.store.Create(r.Context(), p)
	if err != nil {
		var dup *adapters.ErrDuplicateSKU
		if errors.As(err, &dup) {
			httpkit.JSONProblemDetails(w, http.StatusConflict, "product_sku_duplicate",
				"A product with this SKU already exists.",
				map[string]any{fieldExistingID: dup.ExistingID, fieldField: dup.Field})
			return
		}
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONCreatedAt(w, created, "/products/"+created.ID)
}

func (h *ProductHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	p, err := h.store.Get(r.Context(), id, httpkit.WorkspaceID(r))
	writeGetResult(w, p, err)
}

func (h *ProductHandler) list(w http.ResponseWriter, r *http.Request) {
	listResults(w, r, h.store.List)
}

func (h *ProductHandler) update(w http.ResponseWriter, r *http.Request, id string) {
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
	p, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
	if err != nil {
		var dup *adapters.ErrDuplicateSKU
		if errors.As(err, &dup) {
			httpkit.JSONProblemDetails(w, http.StatusConflict, "product_sku_duplicate",
				"A product with this SKU already exists.",
				map[string]any{fieldExistingID: dup.ExistingID, fieldField: dup.Field})
			return
		}
	}
	httpkit.WriteUpdateResult(w, p, err)
}

func (h *ProductHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	p, err := h.store.Archive(r.Context(), id, httpkit.WorkspaceID(r))
	writeGetResult(w, p, err)
}
