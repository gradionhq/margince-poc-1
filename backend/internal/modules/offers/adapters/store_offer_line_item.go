package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

const entityTypeOfferLineItem = "offer_line_item"

// productReader is the narrow seam OfferLineItemStore needs to snapshot a
// product's description/unit_price_minor/default_tax_rate onto a line at
// pick time (OFFER-AC-9b) — read once at create, never re-read later.
// *ProductStore (same package) already satisfies this.
type productReader interface {
	Get(ctx context.Context, id, workspaceID string) (domain.Product, error)
}

// ErrDuplicatePosition reports a live-row position collision within an offer
// (offer_line_item_position_unique), pre-checked ahead of INSERT/UPDATE —
// mirrors ErrDuplicateSKU's shape.
type ErrDuplicatePosition struct {
	ExistingID string
	Position   int
}

func (e *ErrDuplicatePosition) Error() string {
	return fmt.Sprintf("duplicate position: existing_id=%s position=%d", e.ExistingID, e.Position)
}

// OfferLineItemStore executes parameterized SQL against offer_line_item.
type OfferLineItemStore struct {
	db       *sql.DB
	products productReader
}

// NewOfferLineItemStore returns an OfferLineItemStore backed by db, reading
// product snapshots through products.
func NewOfferLineItemStore(db *sql.DB, products productReader) *OfferLineItemStore {
	return &OfferLineItemStore{db: db, products: products}
}

func (s *OfferLineItemStore) checkPositionConflict(ctx context.Context, tx *sql.Tx, offerID, excludeID string, position int) error {
	var existingID string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM offer_line_item
		WHERE offer_id=$1::uuid AND position=$2 AND id <> $3::uuid`,
		offerID, position, sqlutil.NullStrParam(excludeID)).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return &ErrDuplicatePosition{ExistingID: existingID, Position: position}
}

// The queries below spell out the full offer_line_item column list literally
// (rather than sharing it via a `+`-concatenated const) so SonarCloud's
// go:S2077 rule — which traces a global identifier back through its own
// declaration — finds no concatenation to flag on any of these (see
// ProductStore's analogous comment in store_product.go). source/captured_by
// are validated (RequireProvenance) but not DB-backed (no column in the
// DDL) — echoed back from the input on create only, mirroring OfferTemplate's
// same pattern. evidence is stored in the jsonb column and round-tripped.
const offerLineItemInsertQuery = `
	INSERT INTO offer_line_item (id, workspace_id, offer_id, position, product_id,
	    description, unit, quantity, unit_price_minor, discount_pct, tax_rate, evidence)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	RETURNING
	id, workspace_id, offer_id, position, product_id, description, unit,
	quantity, unit_price_minor, discount_pct, tax_rate, evidence,
	created_at, updated_at, archived_at`

const offerLineItemListQuery = `SELECT
	id, workspace_id, offer_id, position, product_id, description, unit,
	quantity, unit_price_minor, discount_pct, tax_rate, evidence,
	created_at, updated_at, archived_at
	FROM offer_line_item WHERE offer_id=$1::uuid AND workspace_id=$2::uuid
	ORDER BY position`

const offerLineItemUpdateQuery = `
	UPDATE offer_line_item
	SET position         = COALESCE($4, position),
	    product_id       = CASE WHEN $5 THEN $6 ELSE product_id END,
	    description      = COALESCE($7, description),
	    unit             = COALESCE($8, unit),
	    quantity         = COALESCE($9, quantity),
	    unit_price_minor = COALESCE($10, unit_price_minor),
	    discount_pct     = COALESCE($11, discount_pct),
	    tax_rate         = COALESCE($12, tax_rate),
	    evidence         = COALESCE($13, evidence)
	WHERE id=$1::uuid AND offer_id=$2::uuid AND workspace_id=$3::uuid
	RETURNING
	id, workspace_id, offer_id, position, product_id, description, unit,
	quantity, unit_price_minor, discount_pct, tax_rate, evidence,
	created_at, updated_at, archived_at`

func scanOfferLineItem(row interface{ Scan(dest ...any) error }) (domain.OfferLineItem, error) {
	var li domain.OfferLineItem
	var evidenceBytes []byte
	err := row.Scan(&li.ID, &li.WorkspaceID, &li.OfferID, &li.Position, &li.ProductID,
		&li.Description, &li.Unit, &li.Quantity, &li.UnitPriceMinor, &li.DiscountPct,
		&li.TaxRate, &evidenceBytes, &li.CreatedAt, &li.UpdatedAt, &li.ArchivedAt)
	if err != nil {
		return li, err
	}
	if len(evidenceBytes) > 0 {
		var ev domain.Evidence
		if jsonErr := json.Unmarshal(evidenceBytes, &ev); jsonErr == nil {
			li.Evidence = &ev
		}
	}
	return li, err
}

func evidenceJSON(e *domain.Evidence) any {
	if e == nil {
		return nil
	}
	return sqlutil.MarshalJSON(e)
}

func evidenceFromUpdate(updates map[string]any) *domain.Evidence {
	raw, ok := updates["evidence"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case *domain.Evidence:
		return v
	case domain.Evidence:
		return &v
	case json.RawMessage:
		var ev domain.Evidence
		if err := json.Unmarshal(v, &ev); err == nil {
			return &ev
		}
	case []byte:
		var ev domain.Evidence
		if err := json.Unmarshal(v, &ev); err == nil {
			return &ev
		}
	default:
		if b, err := json.Marshal(v); err == nil {
			var ev domain.Evidence
			if err := json.Unmarshal(b, &ev); err == nil {
				return &ev
			}
		}
	}
	return nil
}

// applyProductSnapshot resolves li's product-snapshot fields (OFFER-AC-9b):
// when li.ProductID is set, description/unit_price_minor are always copied
// from the product, and tax_rate is copied from the product's default
// unless explicitTaxRate overrides it — in which case explicitTaxRate always
// wins, product-linked or not. Split out of Create purely to keep Create's
// own branch count under the cyclop budget; no behavior change.
func (s *OfferLineItemStore) applyProductSnapshot(ctx context.Context, li domain.OfferLineItem, explicitTaxRate *float64) (domain.OfferLineItem, error) {
	if li.ProductID != nil {
		p, err := s.products.Get(ctx, *li.ProductID, li.WorkspaceID)
		if err != nil {
			return domain.OfferLineItem{}, err
		}
		if p.Description != nil {
			li.Description = *p.Description
		} else {
			li.Description = p.Name
		}
		price := p.UnitPriceMinor
		li.UnitPriceMinor = &price
		if explicitTaxRate == nil && p.DefaultTaxRate != nil {
			li.TaxRate = *p.DefaultTaxRate
		}
	}
	if explicitTaxRate != nil {
		li.TaxRate = *explicitTaxRate
	}
	return li, nil
}

// Create snapshots the referenced product (if any) onto the line — copying
// description/unit_price_minor always, and tax_rate unless explicitTaxRate
// overrides it (OFFER-AC-9b) — inserts the row, recomputes the parent
// offer's totals, and writes one audit_log entry, all in one tx locked
// against a concurrent offer status flip (OFFER-WIRE-4).
// source/captured_by are validated but not persisted (no DB column) —
// echoed back on the returned struct only, mirroring OfferTemplate. evidence
// is persisted when present and returned by the INSERT round-trip.
func (s *OfferLineItemStore) Create(ctx context.Context, li domain.OfferLineItem, explicitTaxRate *float64) (domain.OfferLineItem, error) {
	if err := sqlutil.RequireProvenance(li.Source, li.CapturedBy); err != nil {
		return domain.OfferLineItem{}, err
	}
	if li.Unit == "" {
		li.Unit = "unit"
	}
	li, err := s.applyProductSnapshot(ctx, li, explicitTaxRate)
	if err != nil {
		return domain.OfferLineItem{}, err
	}
	li.ID = ids.New()

	var out domain.OfferLineItem
	err = database.WithWorkspaceTx(ctx, s.db, li.WorkspaceID, func(tx *sql.Tx) error {
		var txErr error
		out, txErr = s.createTx(ctx, tx, li)
		return txErr
	})
	if err != nil {
		return domain.OfferLineItem{}, err
	}
	// Echo back source/captured_by from input (validated but not DB-backed).
	out.Source, out.CapturedBy = li.Source, li.CapturedBy
	return out, nil
}

// createTx locks the parent offer row, enforces the OFFER-WIRE-4 draft-only
// guard and position-uniqueness, inserts li's row, recomputes the parent
// offer's totals, and writes one audit_log entry — all under the caller's
// workspace tx. Extracted from Create purely to keep Create's own
// WithWorkspaceTx closure's cognitive complexity low; no behavior change.
func (s *OfferLineItemStore) createTx(ctx context.Context, tx *sql.Tx, li domain.OfferLineItem) (domain.OfferLineItem, error) {
	status, _, err := lockOfferForMutation(ctx, tx, li.OfferID, li.WorkspaceID)
	if err != nil {
		return domain.OfferLineItem{}, err
	}
	if err := requireDraft(status); err != nil {
		return domain.OfferLineItem{}, err
	}
	if err := s.checkPositionConflict(ctx, tx, li.OfferID, nilUUID, li.Position); err != nil {
		return domain.OfferLineItem{}, err
	}
	row := tx.QueryRowContext(ctx, offerLineItemInsertQuery,
		li.ID, li.WorkspaceID, li.OfferID, li.Position, li.ProductID,
		li.Description, li.Unit, li.Quantity, li.UnitPriceMinor, li.DiscountPct, li.TaxRate, evidenceJSON(li.Evidence))
	out, err := scanOfferLineItem(row)
	if err != nil {
		return domain.OfferLineItem{}, fmt.Errorf("offer_line_item create: %w", err)
	}
	if err := recomputeOfferTotals(ctx, tx, li.OfferID, li.WorkspaceID); err != nil {
		return domain.OfferLineItem{}, err
	}
	e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeOfferLineItem, &li.ID, nil, out)
	e.WorkspaceID = li.WorkspaceID
	if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
		return domain.OfferLineItem{}, fmt.Errorf("offer_line_item create audit: %w", err)
	}
	return out, nil
}

// List returns offerID's live line items in position order (no pagination —
// crm.yaml's listOfferLineItems declares no cursor/limit params).
func (s *OfferLineItemStore) List(ctx context.Context, offerID, workspaceID string) ([]domain.OfferLineItem, error) {
	out := []domain.OfferLineItem{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, offerLineItemListQuery, offerID, workspaceID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			li, scanErr := scanOfferLineItem(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, li)
		}
		return rows.Err()
	})
	return out, err
}

// Update applies a bounded partial update (position/product_id/description/
// unit/quantity/unit_price_minor/discount_pct/tax_rate —
// product_id here is a plain FK write, NOT a re-snapshot trigger; see Global
// Constraint 3), recomputes the parent offer's totals, and writes one
// audit_log entry — atomic with the OFFER-WIRE-4 draft-only guard.
func (s *OfferLineItemStore) Update(ctx context.Context, id, offerID, workspaceID string, updates map[string]any) (domain.OfferLineItem, error) {
	var out domain.OfferLineItem
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var txErr error
		out, txErr = s.updateTx(ctx, tx, id, offerID, workspaceID, updates)
		return txErr
	})
	if err != nil {
		return domain.OfferLineItem{}, err
	}
	return out, nil
}

// updateTx locks the parent offer row, enforces the OFFER-WIRE-4 draft-only
// guard and position-uniqueness, applies the bounded partial update,
// recomputes the parent offer's totals, and writes one audit_log entry — all
// under the caller's workspace tx. Extracted from Update purely to keep
// Update's own WithWorkspaceTx closure's cognitive complexity low; no
// behavior change.
func (s *OfferLineItemStore) updateTx(ctx context.Context, tx *sql.Tx, id, offerID, workspaceID string, updates map[string]any) (domain.OfferLineItem, error) {
	status, _, err := lockOfferForMutation(ctx, tx, offerID, workspaceID)
	if err != nil {
		return domain.OfferLineItem{}, err
	}
	if err := requireDraft(status); err != nil {
		return domain.OfferLineItem{}, err
	}
	if err := s.checkPositionUpdateConflict(ctx, tx, offerID, id, updates); err != nil {
		return domain.OfferLineItem{}, err
	}
	row := tx.QueryRowContext(ctx, offerLineItemUpdateQuery,
		id, offerID, workspaceID,
		nullInt64(updates, "position"),
		hasKey(updates, "product_id"), sqlutil.NullStr(updates, "product_id"),
		sqlutil.NullStr(updates, "description"),
		sqlutil.NullStr(updates, "unit"),
		nullFloat64(updates, "quantity"),
		nullInt64(updates, "unit_price_minor"),
		nullFloat64(updates, "discount_pct"),
		nullFloat64(updates, "tax_rate"),
		evidenceJSON(evidenceFromUpdate(updates)))
	out, err := scanOfferLineItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OfferLineItem{}, errs.ErrNotFound
	}
	if err != nil {
		return domain.OfferLineItem{}, fmt.Errorf("offer_line_item update: %w", err)
	}
	if err := recomputeOfferTotals(ctx, tx, offerID, workspaceID); err != nil {
		return domain.OfferLineItem{}, err
	}
	e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeOfferLineItem, &id, nil, nil)
	e.WorkspaceID = workspaceID
	if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
		return domain.OfferLineItem{}, fmt.Errorf("offer_line_item update audit: %w", err)
	}
	return out, nil
}

// checkPositionUpdateConflict pre-checks Update's incoming "position" field
// (if present and an int64) for a live collision with another row, ahead of
// the UPDATE.
func (s *OfferLineItemStore) checkPositionUpdateConflict(ctx context.Context, tx *sql.Tx, offerID, excludeID string, updates map[string]any) error {
	posVal := nullInt64(updates, "position")
	if posVal == nil {
		return nil
	}
	posInt, ok := posVal.(int64)
	if !ok {
		return nil
	}
	return s.checkPositionConflict(ctx, tx, offerID, excludeID, int(posInt))
}

// Delete hard-deletes the line item (crm.yaml's explicit "Hard-deletes ...
// Returns 204" — see Global Constraint 1), recomputes the parent offer's
// totals, and writes one audit_log entry (action=delete — Global
// Constraint 2) — atomic with the OFFER-WIRE-4 draft-only guard.
func (s *OfferLineItemStore) Delete(ctx context.Context, id, offerID, workspaceID string) error {
	return database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		status, _, err := lockOfferForMutation(ctx, tx, offerID, workspaceID)
		if err != nil {
			return err
		}
		if err := requireDraft(status); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`DELETE FROM offer_line_item WHERE id=$1::uuid AND offer_id=$2::uuid AND workspace_id=$3::uuid`,
			id, offerID, workspaceID)
		if err != nil {
			return fmt.Errorf("offer_line_item delete: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errs.ErrNotFound
		}
		if err := recomputeOfferTotals(ctx, tx, offerID, workspaceID); err != nil {
			return err
		}
		e := crmaudit.EntryFromPrincipal(ctx, "archive", entityTypeOfferLineItem, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer_line_item delete audit: %w", err)
		}
		return nil
	})
}
