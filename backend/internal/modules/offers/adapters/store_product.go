// Package adapters contains the offers module's SQL-backed store
// implementations for product (OFFER-DDL-1) and offer_template (OFFER-DDL-4).
package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

const entityTypeProduct = "product"

// ErrDuplicateSKU reports a live-row SKU collision within a workspace,
// pre-checked ahead of the INSERT/UPDATE so the client gets a stable
// existing_id/field detail (never a raw uq_product_sku constraint error).
type ErrDuplicateSKU struct {
	ExistingID string
	Field      string
}

func (e *ErrDuplicateSKU) Error() string {
	return fmt.Sprintf("duplicate sku: existing_id=%s field=%s", e.ExistingID, e.Field)
}

func nullFloat64(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case float32:
			return float64(n)
		case nil:
			return nil
		}
	}
	return nil
}

func nullInt64(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		}
	}
	return nil
}

func nullBoolProduct(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return nil
}

// ProductStore executes parameterized SQL against the product table.
type ProductStore struct{ db *sql.DB }

// NewProductStore returns a ProductStore backed by db.
func NewProductStore(db *sql.DB) *ProductStore { return &ProductStore{db: db} }

// checkSKUConflict pre-checks whether sku already collides with another live
// row in workspaceID, excluding excludeID (empty on create). Returns
// *ErrDuplicateSKU on collision, nil otherwise. A nil sku never collides
// (uq_product_sku is a partial index, WHERE sku IS NOT NULL).
func (s *ProductStore) checkSKUConflict(ctx context.Context, tx *sql.Tx, workspaceID, excludeID string, sku *string) error {
	if sku == nil {
		return nil
	}
	var existingID string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM product
		WHERE workspace_id=$1::uuid AND sku=$2 AND archived_at IS NULL AND id <> $3::uuid`,
		workspaceID, *sku, sqlutil.NullStrParam(excludeID)).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return &ErrDuplicateSKU{ExistingID: existingID, Field: "sku"}
}

// Create inserts a product row, its create audit_log entry, in one
// workspace-scoped tx. Rejects a live SKU collision (409, pre-checked — never
// a raw uq_product_sku constraint error) and missing provenance (422) before
// ever executing the INSERT.
func (s *ProductStore) Create(ctx context.Context, p domain.Product) (domain.Product, error) {
	if err := sqlutil.RequireProvenance(p.Source, p.CapturedBy); err != nil {
		return domain.Product{}, err
	}
	p.ID = ids.New()
	unit := p.Unit
	if unit == nil {
		u := "unit"
		unit = &u
	}
	err := database.WithWorkspaceTx(ctx, s.db, p.WorkspaceID, func(tx *sql.Tx) error {
		if err := s.checkSKUConflict(ctx, tx, p.WorkspaceID, "00000000-0000-0000-0000-000000000000", p.SKU); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO product (id, workspace_id, name, sku, description, unit, unit_price_minor,
			    currency, default_tax_rate, active, source, captured_by, version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,COALESCE($9,0),$10,$11,$12,1)`,
			p.ID, p.WorkspaceID, p.Name, p.SKU, p.Description, unit, p.UnitPriceMinor,
			p.Currency, p.DefaultTaxRate, p.Active, p.Source, p.CapturedBy); err != nil {
			return fmt.Errorf("product create: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeProduct, &p.ID, nil, p)
		e.WorkspaceID = p.WorkspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("product create audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Product{}, err
	}
	return s.Get(ctx, p.ID, p.WorkspaceID)
}

const productSelectCols = `
	id, workspace_id, name, sku, description, unit, unit_price_minor, currency,
	default_tax_rate, active, version, source, captured_by, created_at, updated_at, archived_at`

func scanProduct(row interface{ Scan(dest ...any) error }) (domain.Product, error) {
	var p domain.Product
	err := row.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.SKU, &p.Description, &p.Unit,
		&p.UnitPriceMinor, &p.Currency, &p.DefaultTaxRate, &p.Active, &p.Version,
		&p.Source, &p.CapturedBy, &p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt)
	return p, err
}

const productGetQuery = `SELECT ` + productSelectCols + `
	FROM product WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`

const productGetAnyQuery = `SELECT ` + productSelectCols + `
	FROM product WHERE id=$1::uuid AND workspace_id=$2::uuid`

const productListQueryLive = `SELECT ` + productSelectCols + `
	FROM product WHERE workspace_id=$1::uuid AND ($2 = '' OR id::text > $2) AND archived_at IS NULL
	ORDER BY id LIMIT $3`

const productListQueryAll = `SELECT ` + productSelectCols + `
	FROM product WHERE workspace_id=$1::uuid AND ($2 = '' OR id::text > $2)
	ORDER BY id LIMIT $3`

// Get returns one live product by id, workspace-scoped; ErrNotFound if absent
// or archived (archived rows stay reachable via List's include_archived=true,
// not via Get — neither getProduct nor archiveProduct documents an
// archived-fetchable Get, unlike getPerson).
func (s *ProductStore) Get(ctx context.Context, id, workspaceID string) (domain.Product, error) {
	var p domain.Product
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, productGetQuery, id, workspaceID)
		var scanErr error
		p, scanErr = scanProduct(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return p, errs.ErrNotFound
	}
	return p, err
}

// List returns a cursor-paginated slice of products (OFFER-AC-9c: an empty
// catalogue answers 200 with an empty page, never an error).
func (s *ProductStore) List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool) ([]domain.Product, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	out := []domain.Product{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		query := productListQueryAll
		if !includeArchived {
			query = productListQueryLive
		}
		rows, err := tx.QueryContext(ctx, query, workspaceID, cursor, limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			p, scanErr := scanProduct(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

// checkSKUConflictOnUpdate pre-checks Update's incoming "sku" field (if
// present and non-empty) for a live collision with another row, ahead of the
// UPDATE itself.
func (s *ProductStore) checkSKUConflictOnUpdate(ctx context.Context, tx *sql.Tx, workspaceID, id string, updates map[string]any) error {
	skuVal, ok := updates["sku"]
	if !ok {
		return nil
	}
	skuStr, ok := skuVal.(string)
	if !ok || skuStr == "" {
		return nil
	}
	return s.checkSKUConflict(ctx, tx, workspaceID, id, &skuStr)
}

// Update applies a bounded partial update (COALESCE — see plan's Global
// Constraints on the crm.yaml PUT-semantics doc contradiction) using
// standard If-Match optimistic concurrency. Rejects a live SKU collision
// with another row (409, pre-checked) before executing the UPDATE.
func (s *ProductStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Product, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		if err := s.checkSKUConflictOnUpdate(ctx, tx, workspaceID, id, updates); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE product
			SET name             = COALESCE($3, name),
			    sku              = CASE WHEN $4 THEN $5 ELSE sku END,
			    description      = CASE WHEN $6 THEN $7 ELSE description END,
			    unit             = COALESCE($8, unit),
			    unit_price_minor = COALESCE($9, unit_price_minor),
			    currency         = COALESCE($10, currency),
			    default_tax_rate = COALESCE($11, default_tax_rate),
			    active           = COALESCE($12, active),
			    source           = COALESCE($13, source),
			    captured_by      = COALESCE($14, captured_by)
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
			  AND ($15 = 0 OR version = $15)`,
			id, workspaceID,
			sqlutil.NullStr(updates, "name"),
			hasKey(updates, "sku"), sqlutil.NullStr(updates, "sku"),
			hasKey(updates, "description"), sqlutil.NullStr(updates, "description"),
			sqlutil.NullStr(updates, "unit"),
			nullInt64(updates, "unit_price_minor"),
			sqlutil.NullStr(updates, "currency"),
			nullFloat64(updates, "default_tax_rate"),
			nullBoolProduct(updates, "active"),
			sqlutil.NullStr(updates, "source"),
			sqlutil.NullStr(updates, "captured_by"),
			ifMatch)
		if err != nil {
			return fmt.Errorf("product update: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if ifMatch != 0 {
				return errs.ErrVersionSkew
			}
			return errs.ErrNotFound
		}
		e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeProduct, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("product update audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Product{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes a product (sets archived_at); a repeat archive is a
// no-op (matches relationship/activity Archive precedent).
func (s *ProductStore) Archive(ctx context.Context, id, workspaceID string) (domain.Product, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE product SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		e := crmaudit.EntryFromPrincipal(ctx, "archive", entityTypeProduct, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("product archive audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Product{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

func (s *ProductStore) getAny(ctx context.Context, id, workspaceID string) (domain.Product, error) {
	var p domain.Product
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, productGetAnyQuery, id, workspaceID)
		var scanErr error
		p, scanErr = scanProduct(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return p, errs.ErrNotFound
	}
	return p, err
}

func hasKey(m map[string]any, key string) bool { _, ok := m[key]; return ok }
