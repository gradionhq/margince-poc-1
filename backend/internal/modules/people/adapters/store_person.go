// Package adapters — PersonStore core (Create, Get, GetAny, List, Update) for
// the people module, extracted from directory/store.go (WS-E-a, D6).
// Additional PersonStore methods are in:
//   - store_person_archive.go  Archive
//   - store_person_dedupe.go   fuzzyDedupe + candidate helpers
//   - store_person_restore.go  Restore
//   - store_merge_person.go    Merge
//   - store_strength.go        StrengthBreakdown + attach helpers + listByStrength
package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/people/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/platform/customfields"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/dedupe"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

// ---------------------------------------------------------------------------
// package-level constants used across adapters files
// ---------------------------------------------------------------------------

const (
	entityTypePerson  = "person"
	fieldPersonID     = "person_id"
	fieldMergedIntoID = "merged_into_id"
)

// Generic, domain-free store helpers (provenance guard, JSON (un)marshalling,
// bounded-update field readers) live in the Tier-0 shared/kernel/sqlutil package.

// ---------------------------------------------------------------------------
// ErrDuplicateEmail
// ---------------------------------------------------------------------------

// ErrDuplicateEmail reports a normalized-email collision during Create
// (PO-AC-16). Mirrors OrgStore's ErrDuplicateDomain.
type ErrDuplicateEmail struct {
	ExistingID string
	Field      string
}

func (e *ErrDuplicateEmail) Error() string {
	return fmt.Sprintf("duplicate email: existing_id=%s field=%s", e.ExistingID, e.Field)
}

// ---------------------------------------------------------------------------
// PersonStore
// ---------------------------------------------------------------------------

// PersonStore executes parameterized SQL against the person table.
type PersonStore struct{ db *sql.DB }

// NewPersonStore returns a PersonStore backed by db.
func NewPersonStore(db *sql.DB) *PersonStore { return &PersonStore{db: db} }

// Create inserts a new person row, overwriting the ID with a fresh one. The
// row INSERT, optional email rows, and audit_log entry run in one
// workspace-scoped tx (margince_app + app.workspace_id) so they commit
// atomically under RLS. Pass nil for emails when the caller does not supply
// any (PO-AC-16).
func (s *PersonStore) Create(ctx context.Context, p domain.Person, emails []domain.PersonEmailInput) (domain.Person, error) {
	if err := sqlutil.RequireProvenance(p.Source, p.CapturedBy); err != nil {
		return domain.Person{}, err
	}
	if p.ID == "" {
		p.ID = ids.New()
	}
	social := sqlutil.MarshalJSON(p.Social)
	address := sqlutil.MarshalJSON(p.Address)
	active, err := customfields.ActiveColumns(ctx, s.db, p.WorkspaceID, "person")
	if err != nil {
		return domain.Person{}, err
	}
	customCols, customVals, customArgs := personCustomInsert(active, p.CustomFields, 12)
	var reviewFlag *dedupe.ReviewFlag
	err = database.WithWorkspaceTx(ctx, s.db, p.WorkspaceID, func(tx *sql.Tx) error {
		args := []any{p.ID, p.WorkspaceID, p.FullName, p.FirstName, p.LastName, p.Title, p.OwnerID, social, address, p.Source, p.CapturedBy}
		args = append(args, customArgs...)
		//nolint:gosec // G202: customCols/customVals are quoted, catalog-derived identifiers + bound-param placeholders ($N), never user input; all values are passed via args
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO person (id, workspace_id, full_name, first_name, last_name, title,
			    owner_id, social, address, source, captured_by`+customCols+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11`+customVals+`)`,
			args...); err != nil {
			return err
		}
		if err := insertPersonEmails(ctx, tx, p.WorkspaceID, p.ID, p.Source, p.CapturedBy, emails); err != nil {
			return err
		}
		// PO-AC-19: the fuzzy tier only runs once the exact-key email tier has
		// already succeeded (no 409) — a non-blocking review-flag, never an
		// error; create still succeeds either way.
		flag, err := s.fuzzyDedupe(ctx, tx, p.WorkspaceID, p.ID, p.FullName, emails)
		if err != nil {
			return err
		}
		reviewFlag = flag
		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypePerson, &p.ID, nil, p)
		e.WorkspaceID = p.WorkspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("person create audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Person{}, err
	}
	created, err := s.Get(ctx, p.ID, p.WorkspaceID)
	if err != nil {
		return domain.Person{}, err
	}
	created.ReviewFlag = reviewFlag
	return created, nil
}

// insertPersonEmails writes createPerson's emails[] rows, 409-ing on the first
// email that already maps to another live person in the workspace
// (uq_person_email, PO-AC-16). source/captured_by come from the person row
// itself — email rows share the parent's provenance, there is no separate
// per-email capture UI yet.
func insertPersonEmails(ctx context.Context, tx *sql.Tx, workspaceID, personID, source, capturedBy string, emails []domain.PersonEmailInput) error {
	for i, e := range emails {
		normalized := strings.ToLower(strings.TrimSpace(e.Email))
		var existingID string
		scanErr := tx.QueryRowContext(ctx, `
			SELECT person_id FROM person_email
			WHERE workspace_id=$1::uuid AND lower(email)=$2 AND archived_at IS NULL`,
			workspaceID, normalized).Scan(&existingID)
		if scanErr == nil {
			return &ErrDuplicateEmail{ExistingID: existingID, Field: fmt.Sprintf("emails[%d].email", i)}
		}
		if !errors.Is(scanErr, sql.ErrNoRows) {
			return scanErr
		}
		emailType := e.EmailType
		if emailType == "" {
			emailType = "work"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO person_email (workspace_id, person_id, email, email_type, is_primary, position, source, captured_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			workspaceID, personID, normalized, emailType, e.IsPrimary, e.Position, source, capturedBy); err != nil {
			return fmt.Errorf("person create email: %w", err)
		}
	}
	return nil
}

// Get returns a live person by ID + workspace.
//
//nolint:dupl // parallel per-entity CRUD: the SQL column list and Scan targets differ by type; a generic extraction would read worse than the explicit form
func (s *PersonStore) Get(ctx context.Context, id, workspaceID string) (domain.Person, error) {
	var p domain.Person
	var socialRaw, addrRaw []byte
	active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, "person")
	if err != nil {
		return p, err
	}
	dests := customfields.ScanDests(active)
	scanArgs := append([]any{
		&p.ID, &p.WorkspaceID, &p.FullName, &p.FirstName, &p.LastName, &p.Title,
		&p.OwnerID, &socialRaw, &addrRaw, &p.MergedIntoID, &p.ConvertedFromLeadID,
	}, dests...)
	scanArgs = append(scanArgs, &p.Version, &p.Source, &p.CapturedBy, &p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt)
	err = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		//nolint:gosec // G201: personCustomSelect returns quoted, catalog-derived identifiers only, never user input
		query := fmt.Sprintf(`
			SELECT id, workspace_id, full_name, first_name, last_name, title,
			       owner_id, social, address, merged_into_id, converted_from_lead_id%s,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`, personCustomSelect(active))
		row := tx.QueryRowContext(ctx, query, id, workspaceID)
		if err := row.Scan(scanArgs...); err != nil {
			return err
		}
		if err := s.attachStrength(ctx, tx, workspaceID, []*domain.Person{&p}); err != nil {
			return err
		}
		return s.attachLastActivity(ctx, tx, workspaceID, []*domain.Person{&p})
	})
	if errors.Is(err, sql.ErrNoRows) {
		return p, errs.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Social = map[string]any{}
	sqlutil.UnmarshalJSON(socialRaw, &p.Social)
	if addrRaw != nil {
		p.Address = map[string]any{}
		sqlutil.UnmarshalJSON(addrRaw, &p.Address)
	}
	p.CustomFields = customfields.ExtractValues(active, dests)
	return p, nil
}

// GetAny returns a person by ID + workspace regardless of archived state
// (crm.yaml getPerson: "Fetchable by id even when archived"), mirroring
// OrgStore.GetAny. Other callers (list/update/merge) keep using the
// live-only Get — this is only for the single-record detail-read path.
//
//nolint:dupl // parallel per-entity CRUD: the SQL column list and Scan targets differ by type; a generic extraction would read worse than the explicit form
func (s *PersonStore) GetAny(ctx context.Context, id, workspaceID string) (domain.Person, error) {
	var p domain.Person
	var socialRaw, addrRaw []byte
	active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, "person")
	if err != nil {
		return p, err
	}
	dests := customfields.ScanDests(active)
	scanArgs := append([]any{
		&p.ID, &p.WorkspaceID, &p.FullName, &p.FirstName, &p.LastName, &p.Title,
		&p.OwnerID, &socialRaw, &addrRaw, &p.MergedIntoID, &p.ConvertedFromLeadID,
	}, dests...)
	scanArgs = append(scanArgs, &p.Version, &p.Source, &p.CapturedBy, &p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt)
	err = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		//nolint:gosec // G201: personCustomSelect returns quoted, catalog-derived identifiers only, never user input
		query := fmt.Sprintf(`
			SELECT id, workspace_id, full_name, first_name, last_name, title,
			       owner_id, social, address, merged_into_id, converted_from_lead_id%s,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid`, personCustomSelect(active))
		row := tx.QueryRowContext(ctx, query, id, workspaceID)
		if err := row.Scan(scanArgs...); err != nil {
			return err
		}
		if err := s.attachStrength(ctx, tx, workspaceID, []*domain.Person{&p}); err != nil {
			return err
		}
		return s.attachLastActivity(ctx, tx, workspaceID, []*domain.Person{&p})
	})
	if errors.Is(err, sql.ErrNoRows) {
		return p, errs.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Social = map[string]any{}
	sqlutil.UnmarshalJSON(socialRaw, &p.Social)
	if addrRaw != nil {
		p.Address = map[string]any{}
		sqlutil.UnmarshalJSON(addrRaw, &p.Address)
	}
	p.CustomFields = customfields.ExtractValues(active, dests)
	return p, nil
}

// List returns a cursor-paginated slice of live persons.
func (s *PersonStore) List(ctx context.Context, workspaceID, cursor string, limit int, sort string) ([]domain.Person, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, "person")
	if err != nil {
		return nil, "", err
	}
	activeNames := make(map[string]struct{}, len(active))
	for _, c := range active {
		activeNames[c.ColumnName] = struct{}{}
	}
	switch sort {
	case "", "id":
		return s.listByID(ctx, workspaceID, cursor, limit, active)
	case "strength":
		return s.listByStrength(ctx, workspaceID, cursor, limit, false)
	case "-strength":
		return s.listByStrength(ctx, workspaceID, cursor, limit, true)
	default:
		key := strings.TrimPrefix(sort, "-")
		if _, ok := activeNames[key]; ok {
			return s.listByCustomColumn(ctx, workspaceID, cursor, limit, sort, active)
		}
		return s.listByID(ctx, workspaceID, cursor, limit, active)
	}
}

func (s *PersonStore) listByID(ctx context.Context, workspaceID, cursor string, limit int, active []customfields.Column) ([]domain.Person, string, error) {
	// Non-nil so an empty result marshals to a JSON array ([]), never null.
	out := []domain.Person{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		//nolint:gosec // G202: personCustomSelect returns quoted, catalog-derived identifiers only, never user input; all values are bound params
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, full_name, first_name, last_name, title,
			       owner_id, social`+personCustomSelect(active)+`,
			       version, source, captured_by, created_at, updated_at
			FROM person
			WHERE workspace_id=$1::uuid AND archived_at IS NULL
			  AND ($2 = '' OR id::text > $2)
			ORDER BY id LIMIT $3`,
			workspaceID, cursor, limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var p domain.Person
			var socialRaw []byte
			dests := customfields.ScanDests(active)
			scanArgs := append([]any{
				&p.ID, &p.WorkspaceID, &p.FullName, &p.FirstName, &p.LastName, &p.Title,
				&p.OwnerID, &socialRaw,
			}, dests...)
			scanArgs = append(scanArgs, &p.Version, &p.Source, &p.CapturedBy,
				&p.CreatedAt, &p.UpdatedAt)
			if err := rows.Scan(scanArgs...); err != nil {
				return err
			}
			p.Social = map[string]any{}
			sqlutil.UnmarshalJSON(socialRaw, &p.Social)
			p.CustomFields = customfields.ExtractValues(active, dests)
			out = append(out, p)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*domain.Person, len(out))
		for i := range out {
			ptrs[i] = &out[i]
		}
		if err := s.attachStrength(ctx, tx, workspaceID, ptrs); err != nil {
			return err
		}
		return s.attachLastActivity(ctx, tx, workspaceID, ptrs)
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

func (s *PersonStore) listByCustomColumn(ctx context.Context, workspaceID, cursor string, limit int, sort string, active []customfields.Column) ([]domain.Person, string, error) {
	column := strings.TrimPrefix(sort, "-")
	desc := strings.HasPrefix(sort, "-")
	// Non-nil so an empty result marshals to a JSON array ([]), never null.
	out := []domain.Person{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		//nolint:gosec // G202: column is re-derived against the caller's own active-columns fetch (never trusted from the transport layer a second time) and pq.QuoteIdentifier-quoted; personCustomSelect returns quoted, catalog-derived identifiers only
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, full_name, first_name, last_name, title,
			       owner_id, social`+personCustomSelect(active)+`,
			       version, source, captured_by, created_at, updated_at
			FROM person
			WHERE workspace_id=$1::uuid AND archived_at IS NULL
			  AND ($2 = '' OR id::text > $2)
			ORDER BY `+pq.QuoteIdentifier(column)+func() string {
			if desc {
				return " DESC NULLS LAST, id"
			}
			return " ASC NULLS LAST, id"
		}()+`
			LIMIT $3`,
			workspaceID, cursor, limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var p domain.Person
			var socialRaw []byte
			dests := customfields.ScanDests(active)
			scanArgs := append([]any{
				&p.ID, &p.WorkspaceID, &p.FullName, &p.FirstName, &p.LastName, &p.Title,
				&p.OwnerID, &socialRaw,
			}, dests...)
			scanArgs = append(scanArgs, &p.Version, &p.Source, &p.CapturedBy,
				&p.CreatedAt, &p.UpdatedAt)
			if err := rows.Scan(scanArgs...); err != nil {
				return err
			}
			p.Social = map[string]any{}
			sqlutil.UnmarshalJSON(socialRaw, &p.Social)
			p.CustomFields = customfields.ExtractValues(active, dests)
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

// Update applies partial updates to a person using optimistic concurrency.
// When ifMatch==0 the version check is skipped (last-write-wins).
func (s *PersonStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Person, error) {
	active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, "person")
	if err != nil {
		return domain.Person{}, err
	}
	err = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var res sql.Result
		var err error
		customSet, customArgs := personCustomUpdate(active, updates, 6)
		base := `
				UPDATE person
				SET full_name  = COALESCE($3, full_name),
				    title      = COALESCE($4, title),
				    owner_id   = COALESCE($5, owner_id)`
		if customSet != "" {
			base += ", " + customSet
		}
		base += ", updated_at = now()"
		if ifMatch == 0 {
			args := []any{
				id, workspaceID,
				sqlutil.NullStr(updates, "full_name"),
				sqlutil.NullStr(updates, "title"),
				sqlutil.NullStr(updates, "owner_id"),
			}
			args = append(args, customArgs...)
			res, err = tx.ExecContext(ctx, base+`
				WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`, args...)
		} else {
			args := []any{
				id, workspaceID,
				sqlutil.NullStr(updates, "full_name"),
				sqlutil.NullStr(updates, "title"),
				sqlutil.NullStr(updates, "owner_id"),
			}
			args = append(args, customArgs...)
			args = append(args, ifMatch)
			res, err = tx.ExecContext(ctx, base+`
				WHERE id=$1::uuid AND workspace_id=$2::uuid AND version=$`+strconv.Itoa(6+len(customArgs))+` AND archived_at IS NULL`, args...)
		}
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if ifMatch != 0 {
				return errs.ErrVersionSkew
			}
			return errs.ErrNotFound
		}
		eu := crmaudit.EntryFromPrincipal(ctx, "update", entityTypePerson, &id, nil, nil)
		eu.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, eu); err != nil {
			return fmt.Errorf("person update audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Person{}, err
	}
	return s.Get(ctx, id, workspaceID)
}
