package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	"github.com/gradionhq/margince/backend/internal/modules/records/ports"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

const entityTypeAttachment = "attachment"

// ErrInvalidScanStatus reports a Scanner verdict outside the allowed
// {clean,blocked} set (never "scanning", the row's own default) — a
// misbehaving scanner seam, surfaced instead of silently persisted.
var ErrInvalidScanStatus = errors.New("invalid scan status")

// AttachmentStore executes parameterized SQL against the attachment table
// (000009 + 000076's scan_status). Bytes never touch this store — only
// metadata + the object-store key (ADR-0051).
type AttachmentStore struct{ db *sql.DB }

// NewAttachmentStore returns an AttachmentStore backed by db.
func NewAttachmentStore(db *sql.DB) *AttachmentStore { return &AttachmentStore{db: db} }

var _ ports.AttachmentStore = (*AttachmentStore)(nil)

// The queries below spell out the full attachment column list literally
// (rather than sharing it via a `+`-concatenated const) so SonarCloud's
// go:S2077 rule finds no concatenation to flag on any of these.
const attachmentGetQuery = `SELECT
	id, workspace_id, entity_type, entity_id, filename, content_type, byte_size,
	storage_key, checksum, scan_status, source, captured_by, created_at, archived_at
	FROM attachment WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`

const attachmentGetAnyQuery = `SELECT
	id, workspace_id, entity_type, entity_id, filename, content_type, byte_size,
	storage_key, checksum, scan_status, source, captured_by, created_at, archived_at
	FROM attachment WHERE id=$1::uuid AND workspace_id=$2::uuid`

const attachmentListQueryLive = `SELECT
	id, workspace_id, entity_type, entity_id, filename, content_type, byte_size,
	storage_key, checksum, scan_status, source, captured_by, created_at, archived_at
	FROM attachment
	WHERE workspace_id=$1::uuid AND ($2 = '' OR id::text > $2)
	  AND ($3 = '' OR entity_type = $3) AND ($4 = '' OR entity_id::text = $4)
	  AND archived_at IS NULL
	ORDER BY id LIMIT $5`

const attachmentListQueryAll = `SELECT
	id, workspace_id, entity_type, entity_id, filename, content_type, byte_size,
	storage_key, checksum, scan_status, source, captured_by, created_at, archived_at
	FROM attachment
	WHERE workspace_id=$1::uuid AND ($2 = '' OR id::text > $2)
	  AND ($3 = '' OR entity_type = $3) AND ($4 = '' OR entity_id::text = $4)
	ORDER BY id LIMIT $5`

func scanAttachment(row interface{ Scan(dest ...any) error }) (domain.Attachment, error) {
	var a domain.Attachment
	err := row.Scan(&a.ID, &a.WorkspaceID, &a.EntityType, &a.EntityID, &a.Filename,
		&a.ContentType, &a.ByteSize, &a.StorageKey, &a.Checksum, &a.ScanStatus,
		&a.Source, &a.CapturedBy, &a.CreatedAt, &a.ArchivedAt)
	return a, err
}

// Create inserts an attachment row and its create audit_log entry (P12) in one
// workspace-scoped tx. A fresh row lands scan_status='scanning' and never
// auto-transitions (RD-PARAM-5) — only MarkScanResult moves it. Rejects
// missing provenance (422) before executing the INSERT.
func (s *AttachmentStore) Create(ctx context.Context, a domain.Attachment) (domain.Attachment, error) {
	if err := sqlutil.RequireProvenance(a.Source, a.CapturedBy); err != nil {
		return domain.Attachment{}, err
	}
	if a.ID == "" {
		a.ID = ids.New()
	}
	if a.ScanStatus == "" {
		a.ScanStatus = domain.ScanStatusScanning
	}
	err := database.WithWorkspaceTx(ctx, s.db, a.WorkspaceID, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO attachment (id, workspace_id, entity_type, entity_id, filename,
			    content_type, byte_size, storage_key, checksum, source, captured_by, scan_status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			a.ID, a.WorkspaceID, a.EntityType, a.EntityID, a.Filename,
			a.ContentType, a.ByteSize, a.StorageKey, a.Checksum, a.Source, a.CapturedBy, a.ScanStatus); err != nil {
			return fmt.Errorf("attachment create: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeAttachment, &a.ID, nil, nil)
		e.WorkspaceID = a.WorkspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("attachment create audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Attachment{}, err
	}
	return s.Get(ctx, a.ID, a.WorkspaceID)
}

// Get returns one live attachment by id, workspace-scoped; ErrNotFound if
// absent or archived (archived rows stay reachable via List's
// include_archived=true, not via Get).
func (s *AttachmentStore) Get(ctx context.Context, id, workspaceID string) (domain.Attachment, error) {
	var a domain.Attachment
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, attachmentGetQuery, id, workspaceID)
		var scanErr error
		a, scanErr = scanAttachment(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return a, errs.ErrNotFound
	}
	return a, err
}

// List returns a cursor-paginated slice of attachments, optionally filtered by
// entity_type/entity_id (each applied only when non-empty). An empty result
// answers with an empty (non-nil) page, never an error.
func (s *AttachmentStore) List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int, includeArchived bool) ([]domain.Attachment, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	out := []domain.Attachment{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		query := attachmentListQueryAll
		if !includeArchived {
			query = attachmentListQueryLive
		}
		rows, err := tx.QueryContext(ctx, query, workspaceID, cursor, entityType, entityID, limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			a, scanErr := scanAttachment(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, a)
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

// Archive soft-deletes an attachment (sets archived_at) and writes its archive
// audit_log entry (P12). A repeat archive is a no-op (matches product/activity
// Archive precedent) and writes no audit row.
func (s *AttachmentStore) Archive(ctx context.Context, id, workspaceID string) (domain.Attachment, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE attachment SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		e := crmaudit.EntryFromPrincipal(ctx, "archive", entityTypeAttachment, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("attachment archive audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Attachment{}, err
	}
	return s.GetAny(ctx, id, workspaceID)
}

// MarkScanResult applies a Scanner verdict to a row's scan_status (RD-PARAM-5).
// It loads the row for its StorageKey, calls scanner.Scan, validates the
// returned status is exactly clean or blocked (ErrInvalidScanStatus otherwise,
// leaving the row unchanged), then persists it. This is the only path off
// 'scanning' — an internal test/admin verdict-apply call, never a public HTTP
// endpoint, so it writes no audit_log row (not a create/update/archive
// mutation the P12 convention covers).
func (s *AttachmentStore) MarkScanResult(ctx context.Context, id, workspaceID string, scanner ports.Scanner) (domain.Attachment, error) {
	row, err := s.GetAny(ctx, id, workspaceID)
	if err != nil {
		return domain.Attachment{}, err
	}
	status, err := scanner.Scan(ctx, row.StorageKey)
	if err != nil {
		return domain.Attachment{}, err
	}
	if status != domain.ScanStatusClean && status != domain.ScanStatusBlocked {
		return domain.Attachment{}, fmt.Errorf("%w: %q", ErrInvalidScanStatus, status)
	}
	err = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, execErr := tx.ExecContext(ctx,
			`UPDATE attachment SET scan_status=$1 WHERE id=$2::uuid AND workspace_id=$3::uuid`,
			status, id, workspaceID)
		return execErr
	})
	if err != nil {
		return domain.Attachment{}, err
	}
	return s.GetAny(ctx, id, workspaceID)
}

// GetAny returns one attachment by id, workspace-scoped, regardless of
// archived_at status (no archived_at IS NULL filter). Used by Archive/
// MarkScanResult to re-fetch the post-mutation row, and by the transport
// single-item GET handler so an archived attachment stays retrievable
// (disclosed-locked 200, matching organizations/GetAny precedent) instead of
// 404ing — archived rows are soft-deleted, not gone.
func (s *AttachmentStore) GetAny(ctx context.Context, id, workspaceID string) (domain.Attachment, error) {
	var a domain.Attachment
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, attachmentGetAnyQuery, id, workspaceID)
		var scanErr error
		a, scanErr = scanAttachment(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return a, errs.ErrNotFound
	}
	return a, err
}
