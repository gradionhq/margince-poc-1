package crmcore

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

// withWorkspaceTx runs fn inside a single tx as the non-superuser margince_app role
// with app.workspace_id set, so FORCE RLS is actually enforced on every CRUD query
// (data-model §1.3). This mirrors ContextGraphStore.withWorkspace — the per-workspace
// CRUD stores were the laggard that queried the bare superuser pool and leaned on the
// app-layer workspace_id predicate alone. fn must use the supplied tx (never the pool)
// so the role+GUC are in scope for its statements.
func withWorkspaceTx(ctx context.Context, db *sql.DB, workspaceID string, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// requireProvenance rejects an empty source or captured_by with a typed sentinel
// (data-model §1.6 provenance). HTTP handlers already reject empties at the edge, but
// non-HTTP callers (import/Datasource/direct store use) must not be able to insert source=""
// or captured_by="" — provenance is a load-bearing invariant, not a nicety.
func requireProvenance(source, capturedBy string) error {
	if source == "" || capturedBy == "" {
		return errs.ErrNullProvenance
	}
	return nil
}

// keyset cursors for non-id orderings -----------------------------------------
//
// A page ordered by (sortCol, id) must seek on the FULL sort key, not id alone —
// otherwise rows with a different sortCol but a smaller/larger id are skipped or
// repeated across pages. We encode both components into one opaque base64 token
// (`<sortVal>|<id>`) so the cursor round-trips the whole key. Person/Org/Deal order
// by id only and keep the plain `id::text > cursor` form — they don't use this.

// encodeKeysetCursor packs (sortVal, id) into one opaque, URL-safe token.
func encodeKeysetCursor(sortVal, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(sortVal + "\x00" + id))
}

// decodeKeysetCursor unpacks a token from encodeKeysetCursor. ok=false for an
// empty or malformed token, in which case the caller treats it as "first page".
func decodeKeysetCursor(cursor string) (sortVal, id string, ok bool) {
	if cursor == "" {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(raw), "\x00", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// nullStrParam binds s as a SQL value, or NULL when s is empty — so an unused
// keyset-seek bound param casts cleanly (NULL::timestamptz) instead of failing on
// an empty-string cast under the short-circuited `NOT $hasCursor OR …` predicate.
func nullStrParam(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func marshalJSON(v any) []byte {
	if v == nil {
		return []byte("{}")
	}
	b, _ := json.Marshal(v)
	return b
}

func unmarshalJSON(raw []byte, dst *map[string]any) {
	if raw == nil {
		return
	}
	_ = json.Unmarshal(raw, dst)
}

func nullStr(m map[string]any, key string) *string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}

func nullTime(m map[string]any, key string) *time.Time {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case *time.Time:
			return t
		case time.Time:
			return &t
		case string:
			parsed, err := time.Parse(time.RFC3339, t)
			if err == nil {
				return &parsed
			}
		}
	}
	return nil
}

// nullBool reads a bool out of an updates map, returning nil (binds SQL NULL,
// leaving a COALESCE target untouched) when the key is absent or not a bool.
func nullBool(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return nil
}

// boolVal safely dereferences a *bool, returning false for nil.
func boolVal(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// ---------------------------------------------------------------------------
// PersonStore
// ---------------------------------------------------------------------------

// PersonStore executes parameterized SQL against the person table.
type PersonStore struct{ db *sql.DB }

// NewPersonStore returns a PersonStore backed by db.
func NewPersonStore(db *sql.DB) *PersonStore { return &PersonStore{db: db} }

// ErrDuplicateEmail reports a live email collision on restore/create.
type ErrDuplicateEmail struct {
	ExistingID string
	Field      string
}

func (e *ErrDuplicateEmail) Error() string {
	return fmt.Sprintf("duplicate email: existing_id=%s field=%s", e.ExistingID, e.Field)
}

// Create inserts a new person row, overwriting the ID with a fresh one. The row
// INSERT and its audit_log entry run in one workspace-scoped tx (margince_app +
// app.workspace_id) so they commit atomically under RLS.
func (s *PersonStore) Create(ctx context.Context, p Person) (Person, error) {
	if err := requireProvenance(p.Source, p.CapturedBy); err != nil {
		return Person{}, err
	}
	if p.ID == "" {
		p.ID = ids.New()
	}
	social := marshalJSON(p.Social)
	address := marshalJSON(p.Address)
	err := withWorkspaceTx(ctx, s.db, p.WorkspaceID, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO person (id, workspace_id, full_name, first_name, last_name, title,
			    owner_id, social, address, source, captured_by, version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,1)`,
			p.ID, p.WorkspaceID, p.FullName, p.FirstName, p.LastName, p.Title,
			p.OwnerID, social, address,
			p.Source, p.CapturedBy); err != nil {
			return err
		}
		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypePerson, &p.ID, nil, p)
		e.WorkspaceID = p.WorkspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("person create audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return Person{}, err
	}
	return s.Get(ctx, p.ID, p.WorkspaceID)
}

// Get returns a live person by ID + workspace.
func (s *PersonStore) Get(ctx context.Context, id, workspaceID string) (Person, error) {
	var p Person
	var socialRaw, addrRaw []byte
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, full_name, first_name, last_name, title,
			       owner_id, social, address, merged_into_id, converted_from_lead_id,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID).Scan(
			&p.ID, &p.WorkspaceID, &p.FullName, &p.FirstName, &p.LastName, &p.Title,
			&p.OwnerID, &socialRaw, &addrRaw, &p.MergedIntoID, &p.ConvertedFromLeadID,
			&p.Version, &p.Source, &p.CapturedBy,
			&p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt,
		)
		if err != nil {
			return err
		}
		if err := s.attachStrength(ctx, tx, workspaceID, []*Person{&p}); err != nil {
			return err
		}
		return s.attachLastActivity(ctx, tx, workspaceID, []*Person{&p})
	})
	if errors.Is(err, sql.ErrNoRows) {
		return p, errs.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Social = map[string]any{}
	unmarshalJSON(socialRaw, &p.Social)
	if addrRaw != nil {
		p.Address = map[string]any{}
		unmarshalJSON(addrRaw, &p.Address)
	}
	return p, nil
}

// List returns a cursor-paginated slice of live persons.
func (s *PersonStore) List(ctx context.Context, workspaceID, cursor string, limit int, sort string) ([]Person, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	switch sort {
	case "", "id":
		return s.listByID(ctx, workspaceID, cursor, limit)
	case "strength":
		return s.listByStrength(ctx, workspaceID, cursor, limit, false)
	case "-strength":
		return s.listByStrength(ctx, workspaceID, cursor, limit, true)
	default:
		return s.listByID(ctx, workspaceID, cursor, limit)
	}
}

func (s *PersonStore) listByID(ctx context.Context, workspaceID, cursor string, limit int) ([]Person, string, error) {
	// Non-nil so an empty result marshals to a JSON array ([]), never null.
	out := []Person{}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, full_name, first_name, last_name, title,
			       owner_id, social, version, source, captured_by, created_at, updated_at
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
			var p Person
			var socialRaw []byte
			if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.FullName, &p.FirstName, &p.LastName, &p.Title,
				&p.OwnerID, &socialRaw, &p.Version, &p.Source, &p.CapturedBy,
				&p.CreatedAt, &p.UpdatedAt); err != nil {
				return err
			}
			p.Social = map[string]any{}
			unmarshalJSON(socialRaw, &p.Social)
			out = append(out, p)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*Person, len(out))
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

// Update applies partial updates to a person using optimistic concurrency.
// When ifMatch==0 the version check is skipped (last-write-wins).
func (s *PersonStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (Person, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var res sql.Result
		var err error
		if ifMatch == 0 {
			res, err = tx.ExecContext(ctx, `
				UPDATE person
				SET full_name  = COALESCE($3, full_name),
				    title      = COALESCE($4, title),
				    owner_id   = COALESCE($5, owner_id),
				    updated_at = now()
				WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
				id, workspaceID,
				nullStr(updates, "full_name"),
				nullStr(updates, "title"),
				nullStr(updates, "owner_id"))
		} else {
			res, err = tx.ExecContext(ctx, `
				UPDATE person
				SET full_name  = COALESCE($3, full_name),
				    title      = COALESCE($4, title),
				    owner_id   = COALESCE($5, owner_id),
				    updated_at = now()
				WHERE id=$1::uuid AND workspace_id=$2::uuid AND version=$6 AND archived_at IS NULL`,
				id, workspaceID,
				nullStr(updates, "full_name"),
				nullStr(updates, "title"),
				nullStr(updates, "owner_id"),
				ifMatch)
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
		return Person{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes a person and returns the archived entity.
func (s *PersonStore) Archive(ctx context.Context, id, workspaceID string) (Person, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE person SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			ea := crmaudit.EntryFromPrincipal(ctx, "archive", entityTypePerson, &id, nil, nil)
			ea.WorkspaceID = workspaceID
			if _, err := crmaudit.WriteTx(ctx, tx, ea); err != nil {
				return fmt.Errorf("person archive audit: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return Person{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// Restore re-activates an archived person, writing one person.restored outbox
// event and one audit_log row in the same workspace-scoped tx. The record must
// already be archived and must not be a merge target.
func (s *PersonStore) Restore(ctx context.Context, id, workspaceID string) (Person, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var archivedAt sql.NullTime
		var mergedInto sql.NullString
		if err := tx.QueryRowContext(ctx, `
			SELECT archived_at, merged_into_id
			FROM person
			WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(&archivedAt, &mergedInto); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errs.ErrNotFound
			}
			return err
		}
		if !archivedAt.Valid {
			return errs.ErrNotArchived
		}
		if mergedInto.Valid {
			return errs.ErrMergedRecord
		}
		if err := s.findDuplicateEmail(ctx, tx, workspaceID, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE person SET archived_at=NULL
			 WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NOT NULL`,
			id, workspaceID); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{fieldPersonID: id})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload)
			 VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "person.restored", id, payload); err != nil {
			return fmt.Errorf("person restore event: %w", err)
		}
		er := crmaudit.EntryFromPrincipal(ctx, "restore", entityTypePerson, &id, nil, nil)
		er.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, er); err != nil {
			return fmt.Errorf("person restore audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return Person{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// getAny fetches a person by id regardless of archived_at status.
func (s *PersonStore) getAny(ctx context.Context, id, workspaceID string) (Person, error) {
	var p Person
	var socialRaw, addrRaw []byte
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, full_name, first_name, last_name, title,
			       owner_id, social, address, merged_into_id, converted_from_lead_id,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(
			&p.ID, &p.WorkspaceID, &p.FullName, &p.FirstName, &p.LastName, &p.Title,
			&p.OwnerID, &socialRaw, &addrRaw, &p.MergedIntoID, &p.ConvertedFromLeadID,
			&p.Version, &p.Source, &p.CapturedBy,
			&p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return p, errs.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Social = map[string]any{}
	unmarshalJSON(socialRaw, &p.Social)
	if addrRaw != nil {
		p.Address = map[string]any{}
		unmarshalJSON(addrRaw, &p.Address)
	}
	return p, nil
}

func (s *PersonStore) findDuplicateEmail(ctx context.Context, tx *sql.Tx, workspaceID, personID string) error {
	var existingID string
	if err := tx.QueryRowContext(ctx, `
		SELECT pe2.person_id
		FROM person_email pe1
		JOIN person_email pe2 ON lower(pe2.email) = lower(pe1.email)
		  AND pe2.workspace_id = pe1.workspace_id
		  AND pe2.person_id <> pe1.person_id
		  AND pe2.archived_at IS NULL
		WHERE pe1.person_id=$1::uuid AND pe1.workspace_id=$2::uuid AND pe1.archived_at IS NULL
		LIMIT 1`,
		personID, workspaceID).Scan(&existingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	return &ErrDuplicateEmail{ExistingID: existingID, Field: "email"}
}
