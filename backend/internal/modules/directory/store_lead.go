package crmcore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lib/pq"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// ---------------------------------------------------------------------------
// LeadStore
// ---------------------------------------------------------------------------

// LeadStore manages lead rows including the promote transaction.
type LeadStore struct{ db *sql.DB }

// NewLeadStore returns a LeadStore.
func NewLeadStore(db *sql.DB) *LeadStore { return &LeadStore{db: db} }

// ErrLeadEmailDuplicate is returned by LeadStore.Create when a non-archived lead
// with the same (workspace_id, lower(email)) already exists (uq_lead_email index).
// ExistingID is the conflicting lead's ID so callers can return a 409 + existing_id.
type ErrLeadEmailDuplicate struct {
	ExistingID string
}

func (e *ErrLeadEmailDuplicate) Error() string {
	return fmt.Sprintf("lead email duplicate: existing id %s", e.ExistingID)
}

// emitLeadEvent inserts exactly one lead lifecycle event into the outbox within
// the caller's transaction, so a lead mutation and its event commit atomically.
// Topics: lead.created, lead.updated, lead.disqualified (events.md).
func emitLeadEvent(ctx context.Context, tx *sql.Tx, workspaceID, topic, leadID string) error {
	payload, _ := json.Marshal(map[string]any{"lead_id": leadID})
	_, err := tx.ExecContext(ctx,
		`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
		workspaceID, topic, leadID, payload)
	return err
}

// Create inserts a lead in one workspace-scoped tx.
func (s *LeadStore) Create(ctx context.Context, l Lead) (Lead, error) {
	if err := requireProvenance(l.Source, l.CapturedBy); err != nil {
		return Lead{}, err
	}
	l.ID = ids.New()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Lead{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return Lead{}, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, l.WorkspaceID); err != nil {
		return Lead{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO lead (id, workspace_id, full_name, email, title, company_name,
		    candidate_org_key, status, score, owner_id, source_system, source_id,
		    source, captured_by, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,1)`,
		l.ID, l.WorkspaceID, l.FullName, l.Email, l.Title, l.CompanyName,
		l.CandidateOrgKey, l.Status, l.Score, l.OwnerID, l.SourceSystem, l.SourceID,
		l.Source, l.CapturedBy); err != nil {
		// Detect uq_lead_email unique-index violation and return a typed error
		// carrying the existing lead ID so the handler can emit 409 + existing_id.
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.Constraint == "uq_lead_email" {
			existingID := s.findByEmail(ctx, l.WorkspaceID, l.Email)
			return Lead{}, &ErrLeadEmailDuplicate{ExistingID: existingID}
		}
		return Lead{}, err
	}
	if err := emitLeadEvent(ctx, tx, l.WorkspaceID, "lead.created", l.ID); err != nil {
		return Lead{}, err
	}
	if err := tx.Commit(); err != nil {
		return Lead{}, err
	}
	return s.Get(ctx, l.ID, l.WorkspaceID)
}

// findByEmail returns the ID of the first non-archived lead with the given email
// in the workspace. Returns "" if not found. Used to populate ErrLeadEmailDuplicate.
func (s *LeadStore) findByEmail(ctx context.Context, workspaceID string, email *string) string {
	if email == nil || *email == "" {
		return ""
	}
	var id string
	_ = withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(
			ctx,
			`SELECT id FROM lead WHERE workspace_id=$1 AND lower(email)=lower($2) AND archived_at IS NULL LIMIT 1`,
			workspaceID, *email,
		).Scan(&id)
	})
	return id
}

// Get returns a live lead by id, workspace-scoped; ErrNotFound if absent.
//
//nolint:dupl // parallel per-entity CRUD: the SQL column list and Scan targets differ by type; a generic extraction would read worse than the explicit form
func (s *LeadStore) Get(ctx context.Context, id, workspaceID string) (Lead, error) {
	var l Lead
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, full_name, email, title, company_name,
			       candidate_org_key, status, score, owner_id, promoted_person_id, promoted_at,
			       source_system, source_id, version, source, captured_by,
			       created_at, updated_at, archived_at
			FROM lead WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID).Scan(
			&l.ID, &l.WorkspaceID, &l.FullName, &l.Email, &l.Title, &l.CompanyName,
			&l.CandidateOrgKey, &l.Status, &l.Score, &l.OwnerID, &l.PromotedPersonID, &l.PromotedAt,
			&l.SourceSystem, &l.SourceID, &l.Version, &l.Source, &l.CapturedBy,
			&l.CreatedAt, &l.UpdatedAt, &l.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return l, errs.ErrNotFound
	}
	return l, err
}

// List returns a keyset page of leads for the workspace and the next cursor.
func (s *LeadStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]Lead, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// ORDER BY score DESC, id — seek the full (score, id) key so a page boundary
	// between equal-score leads neither skips nor repeats rows.
	curScore, curID, hasCursor := decodeKeysetCursor(cursor)

	var out []Lead
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, full_name, email, title, company_name,
			       status, score, owner_id, version, source, captured_by, created_at, updated_at
			FROM lead
			WHERE workspace_id=$1::uuid AND archived_at IS NULL
			  AND (NOT $2 OR (score, id) < ($3::int, $4::uuid))
			ORDER BY score DESC, id DESC LIMIT $5`,
			workspaceID, hasCursor, nullStrParam(curScore), nullStrParam(curID), limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var l Lead
			if err := rows.Scan(&l.ID, &l.WorkspaceID, &l.FullName, &l.Email, &l.Title, &l.CompanyName,
				&l.Status, &l.Score, &l.OwnerID, &l.Version,
				&l.Source, &l.CapturedBy,
				&l.CreatedAt, &l.UpdatedAt); err != nil {
				return err
			}
			out = append(out, l)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(out) > limit {
		last := out[limit-1]
		next = encodeKeysetCursor(strconv.Itoa(last.Score), last.ID)
		out = out[:limit]
	}
	return out, next, nil
}

// Update applies partial updates to a lead.
// When ifMatch==0 the version check is skipped (last-write-wins).
func (s *LeadStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (Lead, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Lead{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return Lead{}, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return Lead{}, err
	}

	var res sql.Result
	if ifMatch == 0 {
		res, err = tx.ExecContext(ctx, `
			UPDATE lead
			SET status     = COALESCE($3, status),
			    owner_id   = COALESCE($4::uuid, owner_id),
			    updated_at = now()
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID,
			nullStr(updates, "status"),
			nullStr(updates, "owner_id"))
	} else {
		res, err = tx.ExecContext(ctx, `
			UPDATE lead
			SET status     = COALESCE($3, status),
			    owner_id   = COALESCE($4::uuid, owner_id),
			    updated_at = now()
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND version=$5 AND archived_at IS NULL`,
			id, workspaceID,
			nullStr(updates, "status"),
			nullStr(updates, "owner_id"),
			ifMatch)
	}
	if err != nil {
		return Lead{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if ifMatch != 0 {
			return Lead{}, errs.ErrVersionSkew
		}
		return Lead{}, errs.ErrNotFound
	}
	if err := emitLeadEvent(ctx, tx, workspaceID, "lead.updated", id); err != nil {
		return Lead{}, err
	}
	if err := tx.Commit(); err != nil {
		return Lead{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive disqualifies a lead: it sets status='disqualified' + archived_at (soft
// delete — retained for audit, still fetchable by id) and emits exactly one
// lead.disqualified event, atomically. A no-op re-archive emits no event.
func (s *LeadStore) Archive(ctx context.Context, id, workspaceID string) (Lead, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Lead{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return Lead{}, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return Lead{}, err
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE lead SET status='disqualified', archived_at=now()
		 WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
		id, workspaceID)
	if err != nil {
		return Lead{}, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		if err := emitLeadEvent(ctx, tx, workspaceID, "lead.disqualified", id); err != nil {
			return Lead{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Lead{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// getAny fetches a lead by id regardless of archived_at status.
//
//nolint:dupl // parallel per-entity CRUD: the SQL column list and Scan targets differ by type; a generic extraction would read worse than the explicit form
func (s *LeadStore) getAny(ctx context.Context, id, workspaceID string) (Lead, error) {
	var l Lead
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, full_name, email, title, company_name,
			       candidate_org_key, status, score, owner_id, promoted_person_id, promoted_at,
			       source_system, source_id, version, source, captured_by,
			       created_at, updated_at, archived_at
			FROM lead WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(
			&l.ID, &l.WorkspaceID, &l.FullName, &l.Email, &l.Title, &l.CompanyName,
			&l.CandidateOrgKey, &l.Status, &l.Score, &l.OwnerID, &l.PromotedPersonID, &l.PromotedAt,
			&l.SourceSystem, &l.SourceID, &l.Version, &l.Source, &l.CapturedBy,
			&l.CreatedAt, &l.UpdatedAt, &l.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return l, errs.ErrNotFound
	}
	return l, err
}

// Promote converts a lead into a person in a single transaction:
//  1. Creates a new person (with converted_from_lead_id set).
//  2. Relinks activity_link rows from lead to person (if any exist — leads don't have
//     native activity_link entries, but any linked via a future extension are relinked).
//  3. Sets lead status=promoted + promoted_person_id + promoted_at.
//  4. Writes an audit_log row.
//
// Returns the newly-created Person.
func (s *LeadStore) Promote(ctx context.Context, leadID, workspaceID, actorID string) (Person, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Person{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	// Run as the non-superuser app role with the workspace GUC set, so FORCE RLS is
	// enforced for the whole promotion (lead read, person insert, lead update, audit).
	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return Person{}, fmt.Errorf("promote set role: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id',$1,true)`, workspaceID); err != nil {
		return Person{}, fmt.Errorf("promote set guc: %w", err)
	}

	// 1. Load lead
	var l Lead
	err = tx.QueryRowContext(ctx, `
		SELECT id, workspace_id, full_name, email, title, company_name,
		       status, source, captured_by
		FROM lead WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
		leadID, workspaceID).Scan(
		&l.ID, &l.WorkspaceID, &l.FullName, &l.Email, &l.Title, &l.CompanyName,
		&l.Status, &l.Source, &l.CapturedBy,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Person{}, errs.ErrNotFound
	}
	if err != nil {
		return Person{}, err
	}
	if l.Status == actionPromoted {
		return Person{}, errs.ErrConflict
	}

	// 2. Create person
	personID := ids.New()
	fullName := "Lead " + leadID
	if l.FullName != nil && *l.FullName != "" {
		fullName = *l.FullName
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO person (id, workspace_id, full_name, title,
		    converted_from_lead_id, source, captured_by, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,1)`,
		personID, workspaceID, fullName, l.Title,
		leadID, l.Source, l.CapturedBy)
	if err != nil {
		return Person{}, err
	}

	// 3. Set lead status=promoted
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		UPDATE lead
		SET status=            'promoted',
		    promoted_person_id= $1::uuid,
		    promoted_at=        $2,
		    updated_at=         $2
		WHERE id=$3::uuid AND workspace_id=$4::uuid`,
		personID, now, leadID, workspaceID)
	if err != nil {
		return Person{}, err
	}

	// 4. Audit log (in-tx — atomic with the promotion)
	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID: workspaceID, ActorType: "human", ActorID: actorID,
		Action: "promote", EntityType: entityTypeLead, EntityID: &leadID,
	}); err != nil {
		return Person{}, fmt.Errorf("promote audit: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Person{}, err
	}

	// Return the newly created person
	ps := &PersonStore{db: s.db}
	return ps.Get(ctx, personID, workspaceID)
}
