// Package customfields (this file): Rename and Retire — the two catalog-only
// lifecycle mutations that never run DDL (see create.go's role-switch note
// for why SetOptions, in options.go, needs the base-role-then-downgrade
// shape and these two don't). Both write exactly one 'update' audit row
// (CF-T04 Context: reusing 'update' avoids a new audit_log_action_check
// migration — the CHECK already allows it, matching updateActivity/generic
// update precedent elsewhere).
package customfields

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// ErrNotFound is returned by Rename/Retire/SetOptions when no catalog row
// matches (id, workspace_id) — the RLS-scoped SELECT ... FOR UPDATE finds
// nothing, so the HTTP layer maps it to 404, never a 500.
var ErrNotFound = errors.New("customfields: not found")

// scanCatalogRow scans one custom_field row — RETURNING id, workspace_id,
// object, slug, label, type, status, archived_at, column_name, currency,
// options, created_by, created_at, updated_at, version, in that order —
// into a Created, decoding its jsonb options column. Shared by Rename,
// Retire, and SetOptions, each of which spells out that same column list as
// a literal RETURNING clause (rather than sharing it via a `+`-concatenated
// const) so SonarCloud's go:S2077 rule — which flags any query built by
// string concatenation, even of a fixed, non-user-controlled constant —
// finds no concatenation to flag on any of these.
func scanCatalogRow(row *sql.Row) (Created, error) {
	var out Created
	var optionsRaw []byte
	if err := row.Scan(&out.ID, &out.WorkspaceID, &out.Object, &out.Slug, &out.Label, &out.Type, &out.Status, &out.ArchivedAt,
		&out.ColumnName, &out.Currency, &optionsRaw, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt, &out.Version); err != nil {
		return Created{}, err
	}
	if len(optionsRaw) > 0 {
		_ = json.Unmarshal(optionsRaw, &out.Options)
	}
	return out, nil
}

// Rename updates a custom field's catalog label only (CUSTOM-FIELDS-WIRE-3):
// column_name/object/type never move. Catalog-only — no ALTER TABLE — so
// this runs entirely as margince_app inside WithWorkspaceTx, unlike Create
// (create.go) and SetOptions (options.go), which must run DDL as the base
// role first.
func Rename(ctx context.Context, db *sql.DB, id, label string) (Created, error) {
	if strings.TrimSpace(label) == "" {
		return Created{}, &ErrValidation{Errors: []FieldError{{Field: fieldLabel, Code: codeRequired}}}
	}
	p, _ := crmctx.From(ctx)
	if p.TenantID == "" {
		return Created{}, fmt.Errorf("customfields: empty workspace_id")
	}

	var out Created
	err := database.WithWorkspaceTx(ctx, db, p.TenantID, func(tx *sql.Tx) error {
		var oldLabel string
		if err := tx.QueryRowContext(ctx, `SELECT label FROM custom_field WHERE id=$1::uuid AND workspace_id=$2::uuid FOR UPDATE`,
			id, p.TenantID).Scan(&oldLabel); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("customfields: select for rename: %w", err)
		}

		row := tx.QueryRowContext(ctx, `UPDATE custom_field SET label=$1 WHERE id=$2::uuid AND workspace_id=$3::uuid
			RETURNING id, workspace_id, object, slug, label, type, status, archived_at, column_name, currency, options, created_by, created_at, updated_at, version`,
			label, id, p.TenantID)
		updated, err := scanCatalogRow(row)
		if err != nil {
			return fmt.Errorf("customfields: update label: %w", err)
		}
		out = updated

		entID := out.ID
		_, auditErr := crmaudit.WriteTx(ctx, tx, crmaudit.EntryFromPrincipal(ctx, "update", "custom_field", &entID,
			map[string]any{fieldLabel: oldLabel}, map[string]any{fieldLabel: label}))
		return auditErr
	})
	return out, err
}

// Retire flips status to 'retired' — a catalog-only status flip
// (CUSTOM-FIELDS-WIRE-4/AC-13): never alters or drops the physical column,
// archived_at stays null (retire is a status flip, not an archive). No DDL,
// same tx shape as Rename.
func Retire(ctx context.Context, db *sql.DB, id string) (Created, error) {
	p, _ := crmctx.From(ctx)
	if p.TenantID == "" {
		return Created{}, fmt.Errorf("customfields: empty workspace_id")
	}

	var out Created
	err := database.WithWorkspaceTx(ctx, db, p.TenantID, func(tx *sql.Tx) error {
		var oldStatus string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM custom_field WHERE id=$1::uuid AND workspace_id=$2::uuid FOR UPDATE`,
			id, p.TenantID).Scan(&oldStatus); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("customfields: select for retire: %w", err)
		}

		row := tx.QueryRowContext(ctx, `UPDATE custom_field SET status='retired' WHERE id=$1::uuid AND workspace_id=$2::uuid
			RETURNING id, workspace_id, object, slug, label, type, status, archived_at, column_name, currency, options, created_by, created_at, updated_at, version`,
			id, p.TenantID)
		updated, err := scanCatalogRow(row)
		if err != nil {
			return fmt.Errorf("customfields: update status: %w", err)
		}
		out = updated

		entID := out.ID
		_, auditErr := crmaudit.WriteTx(ctx, tx, crmaudit.EntryFromPrincipal(ctx, "update", "custom_field", &entID,
			map[string]any{"status": oldStatus}, map[string]any{"status": "retired"}))
		return auditErr
	})
	return out, err
}
