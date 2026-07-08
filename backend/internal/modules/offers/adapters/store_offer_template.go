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

const entityTypeOfferTemplate = "offer_template"

// ErrDuplicateTemplateName reports a live-row name collision within a
// workspace (offer_template_name_unique), pre-checked ahead of INSERT/UPDATE.
type ErrDuplicateTemplateName struct{ ExistingID string }

func (e *ErrDuplicateTemplateName) Error() string {
	return fmt.Sprintf("duplicate offer_template name: existing_id=%s", e.ExistingID)
}

// ErrDefaultConflict reports an existing is_default=true row for the same
// (workspace, locale) — uq_offer_template_default, pre-checked ahead of
// INSERT/UPDATE (OFFER-DDL-4: at most one default per locale).
type ErrDefaultConflict struct{ ExistingID, Locale string }

func (e *ErrDefaultConflict) Error() string {
	return fmt.Sprintf("default conflict: existing_id=%s locale=%s", e.ExistingID, e.Locale)
}

// OfferTemplateStore executes parameterized SQL against the offer_template
// table. source/captured_by are validated (RequireProvenance) but never
// persisted — see the domain.OfferTemplate doc comment and this plan's
// Global Constraints (offer_template has no such DB columns).
type OfferTemplateStore struct{ db *sql.DB }

// NewOfferTemplateStore returns an OfferTemplateStore backed by db.
func NewOfferTemplateStore(db *sql.DB) *OfferTemplateStore { return &OfferTemplateStore{db: db} }

func (s *OfferTemplateStore) checkNameConflict(ctx context.Context, tx *sql.Tx, workspaceID, excludeID, name string) error {
	var existingID string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM offer_template
		WHERE workspace_id=$1::uuid AND name=$2 AND archived_at IS NULL AND id <> $3::uuid`,
		workspaceID, name, sqlutil.NullStrParam(excludeID)).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return &ErrDuplicateTemplateName{ExistingID: existingID}
}

func (s *OfferTemplateStore) checkDefaultConflict(ctx context.Context, tx *sql.Tx, workspaceID, excludeID, locale string, isDefault bool) error {
	if !isDefault {
		return nil
	}
	var existingID string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM offer_template
		WHERE workspace_id=$1::uuid AND locale=$2 AND is_default AND archived_at IS NULL AND id <> $3::uuid`,
		workspaceID, locale, sqlutil.NullStrParam(excludeID)).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return &ErrDefaultConflict{ExistingID: existingID, Locale: locale}
}

// Create inserts an offer_template row in one workspace-scoped tx, rejecting
// a live name collision (409 offer_template_name_duplicate) or a same-locale
// default collision (409 offer_template_default_conflict) — both pre-checked,
// never a raw constraint error — and missing provenance (422, validated only,
// not persisted).
func (s *OfferTemplateStore) Create(ctx context.Context, t domain.OfferTemplate) (domain.OfferTemplate, error) {
	if err := sqlutil.RequireProvenance(t.Source, t.CapturedBy); err != nil {
		return domain.OfferTemplate{}, err
	}
	t.ID = ids.New()
	locale := t.Locale
	if locale == "" {
		locale = "de-DE"
	}
	layout := sqlutil.MarshalJSON(t.Layout)
	err := database.WithWorkspaceTx(ctx, s.db, t.WorkspaceID, func(tx *sql.Tx) error {
		const nilID = "00000000-0000-0000-0000-000000000000"
		if err := s.checkNameConflict(ctx, tx, t.WorkspaceID, nilID, t.Name); err != nil {
			return err
		}
		if err := s.checkDefaultConflict(ctx, tx, t.WorkspaceID, nilID, locale, t.IsDefault); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO offer_template (id, workspace_id, name, locale, is_default, layout, version)
			VALUES ($1,$2,$3,$4,$5,$6,1)`,
			t.ID, t.WorkspaceID, t.Name, locale, t.IsDefault, layout); err != nil {
			return fmt.Errorf("offer_template create: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeOfferTemplate, &t.ID, nil, t)
		e.WorkspaceID = t.WorkspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer_template create audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.OfferTemplate{}, err
	}
	created, err := s.Get(ctx, t.ID, t.WorkspaceID)
	if err != nil {
		return domain.OfferTemplate{}, err
	}
	// source/captured_by are validated but not DB-backed (no column) — echo
	// the caller's submitted values on the create response only, matching
	// what a client just sent; a subsequent Get/List cannot reproduce them.
	created.Source, created.CapturedBy = t.Source, t.CapturedBy
	return created, nil
}

const offerTemplateSelectCols = `
	id, workspace_id, name, locale, is_default, layout, version, created_at, updated_at, archived_at`

func scanOfferTemplate(row interface{ Scan(dest ...any) error }) (domain.OfferTemplate, error) {
	var t domain.OfferTemplate
	var layoutRaw []byte
	err := row.Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.Locale, &t.IsDefault, &layoutRaw,
		&t.Version, &t.CreatedAt, &t.UpdatedAt, &t.ArchivedAt)
	if err != nil {
		return t, err
	}
	t.Layout = map[string]interface{}{}
	sqlutil.UnmarshalJSON(layoutRaw, &t.Layout)
	return t, nil
}

// Get returns one live offer_template by id, workspace-scoped; ErrNotFound if
// absent or archived.
func (s *OfferTemplateStore) Get(ctx context.Context, id, workspaceID string) (domain.OfferTemplate, error) {
	var t domain.OfferTemplate
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `SELECT `+offerTemplateSelectCols+`
			FROM offer_template WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		var scanErr error
		t, scanErr = scanOfferTemplate(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return t, errs.ErrNotFound
	}
	return t, err
}

// List returns a cursor-paginated slice of offer_templates.
func (s *OfferTemplateStore) List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool) ([]domain.OfferTemplate, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	out := []domain.OfferTemplate{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		where := ""
		if !includeArchived {
			where = " AND archived_at IS NULL"
		}
		rows, err := tx.QueryContext(ctx, `SELECT `+offerTemplateSelectCols+`
			FROM offer_template WHERE workspace_id=$1::uuid AND ($2 = '' OR id::text > $2)`+where+`
			ORDER BY id LIMIT $3`,
			workspaceID, cursor, limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			t, scanErr := scanOfferTemplate(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, t)
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

// Update applies a bounded partial update (COALESCE) with standard If-Match
// optimistic concurrency. Rejects a live name collision or same-locale
// default collision (both pre-checked) before executing the UPDATE.
func (s *OfferTemplateStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.OfferTemplate, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		if nameVal, ok := updates["name"].(string); ok && nameVal != "" {
			if err := s.checkNameConflict(ctx, tx, workspaceID, id, nameVal); err != nil {
				return err
			}
		}
		if isDefaultVal, ok := updates["is_default"].(bool); ok && isDefaultVal {
			locale := ""
			if l, ok := updates["locale"].(string); ok {
				locale = l
			} else {
				if err := tx.QueryRowContext(ctx, `SELECT locale FROM offer_template WHERE id=$1::uuid AND workspace_id=$2::uuid`,
					id, workspaceID).Scan(&locale); err != nil {
					return err
				}
			}
			if err := s.checkDefaultConflict(ctx, tx, workspaceID, id, locale, true); err != nil {
				return err
			}
		}
		var layoutParam any
		if l, ok := updates["layout"]; ok {
			layoutParam = sqlutil.MarshalJSON(l)
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE offer_template
			SET name       = COALESCE($3, name),
			    locale     = COALESCE($4, locale),
			    is_default = COALESCE($5, is_default),
			    layout     = COALESCE($6, layout)
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
			  AND ($7 = 0 OR version = $7)`,
			id, workspaceID,
			sqlutil.NullStr(updates, "name"),
			sqlutil.NullStr(updates, "locale"),
			nullBoolOfferTemplate(updates, "is_default"),
			layoutParam,
			ifMatch)
		if err != nil {
			return fmt.Errorf("offer_template update: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if ifMatch != 0 {
				return errs.ErrVersionSkew
			}
			return errs.ErrNotFound
		}
		e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeOfferTemplate, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer_template update audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.OfferTemplate{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes an offer_template (sets archived_at); a repeat archive
// is a no-op.
func (s *OfferTemplateStore) Archive(ctx context.Context, id, workspaceID string) (domain.OfferTemplate, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE offer_template SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		e := crmaudit.EntryFromPrincipal(ctx, "archive", entityTypeOfferTemplate, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer_template archive audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.OfferTemplate{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

func (s *OfferTemplateStore) getAny(ctx context.Context, id, workspaceID string) (domain.OfferTemplate, error) {
	var t domain.OfferTemplate
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `SELECT `+offerTemplateSelectCols+`
			FROM offer_template WHERE id=$1::uuid AND workspace_id=$2::uuid`, id, workspaceID)
		var scanErr error
		t, scanErr = scanOfferTemplate(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return t, errs.ErrNotFound
	}
	return t, err
}

func nullBoolOfferTemplate(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return nil
}
