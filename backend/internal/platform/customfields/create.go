package customfields

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// ErrStructural is returned when spec's label is judged structural — a new
// object, relationship, or logic, never a bounded scalar attribute
// (CUSTOM-FIELDS-AC-4).
var ErrStructural = errors.New("customfields: structural change refused")

// ErrValidation wraps the closed-set validation failures (see FieldError).
type ErrValidation struct{ Errors []FieldError }

func (e *ErrValidation) Error() string { return "customfields: validation failed" }

// Created is the row Create returns — the catalog fields the HTTP layer
// needs to shape the 201 CustomField response. JSON tags mirror the
// contract's CustomField schema so the handler can marshal it directly
// (the codebase-wide convention: domain structs carry contract-matching
// json tags rather than being converted into the oapi-codegen types package).
type Created struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Object      string    `json:"object"`
	Label       string    `json:"label"`
	Slug        string    `json:"slug"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	ColumnName  string    `json:"column_name"`
	Currency    *string   `json:"currency"`
	Options     []string  `json:"options,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Version     int64     `json:"version"`
}

// Create is the single chokepoint allowed to run a runtime ALTER TABLE
// (custom-fields.md "one chokepoint"): validates spec, refuses a
// structural request, derives slug/column_name, then runs ALTER TABLE +
// the custom_field catalog INSERT + one audit_log row inside one
// transaction — all three land together or the whole thing rolls back.
//
// Deliberately does NOT use database.WithWorkspaceTx: that helper
// downgrades to the margince_app role (DML-only, RLS-forced) BEFORE
// running its callback, but margince_app has no ALTER privilege on core
// tables (000004_app_role.up.sql grants DML only — the pool's base/owning
// role is the only one that can ALTER). So this function begins its own
// tx as the base role, runs the ALTER TABLE FIRST, THEN downgrades via
// database.SetWorkspaceScope for the RLS-governed catalog insert + audit
// write, and only then commits. Postgres DDL is transactional, so a
// failure at either later step still rolls back the ALTER TABLE.
func Create(ctx context.Context, db *sql.DB, spec FieldSpec) (Created, error) {
	if errs := Validate(spec); len(errs) > 0 {
		return Created{}, &ErrValidation{Errors: errs}
	}
	if IsStructural(spec.Label) {
		return Created{}, ErrStructural
	}
	slug := DeriveSlug(spec.Label)
	columnName := ColumnName(slug)
	ddl, err := BuildDDL(spec.Object, columnName, spec)
	if err != nil {
		return Created{}, err
	}

	p, _ := crmctx.From(ctx)
	if p.TenantID == "" {
		return Created{}, fmt.Errorf("customfields: empty workspace_id")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Created{}, fmt.Errorf("customfields: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Step 1: the ALTER TABLE runs as the pool's base (owning) role.
	if _, err := tx.ExecContext(ctx, ddl); err != nil {
		return Created{}, fmt.Errorf("customfields: alter table: %w", err)
	}

	// Step 2: downgrade to margince_app + set the workspace GUC so the
	// catalog insert and audit write are RLS-governed like every other
	// tenant write.
	if err := database.SetWorkspaceScope(ctx, tx, p.TenantID); err != nil {
		return Created{}, fmt.Errorf("customfields: set workspace scope: %w", err)
	}

	var currency *string
	if spec.Type == TypeCurrency {
		c := spec.Currency
		currency = &c
	}
	var optionsJSON []byte
	if spec.Type == TypePicklist {
		optionsJSON, _ = json.Marshal(spec.Options)
	}

	var out Created
	var optionsRaw []byte
	row := tx.QueryRowContext(ctx, `
		INSERT INTO custom_field (workspace_id, object, slug, label, type, column_name, currency, options, created_by)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9::uuid)
		RETURNING id, workspace_id, object, slug, label, type, status, column_name, currency, options, created_by, created_at, updated_at, version`,
		p.TenantID, spec.Object, slug, spec.Label, spec.Type, columnName, currency, optionsJSON, p.UserID)
	if err := row.Scan(&out.ID, &out.WorkspaceID, &out.Object, &out.Slug, &out.Label, &out.Type, &out.Status,
		&out.ColumnName, &out.Currency, &optionsRaw, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt, &out.Version); err != nil {
		return Created{}, fmt.Errorf("customfields: insert catalog row: %w", err)
	}
	if len(optionsRaw) > 0 {
		_ = json.Unmarshal(optionsRaw, &out.Options)
	}

	entID := out.ID
	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.EntryFromPrincipal(ctx, "create", "custom_field", &entID, nil,
		map[string]any{fieldObject: spec.Object, fieldLabel: spec.Label, fieldType: spec.Type, "column_name": columnName, "source": spec.Source, "captured_by": spec.CapturedBy})); err != nil {
		return Created{}, fmt.Errorf("customfields: audit write: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Created{}, fmt.Errorf("customfields: commit: %w", err)
	}
	return out, nil
}
