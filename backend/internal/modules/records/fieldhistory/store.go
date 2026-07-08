package fieldhistory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	audithistorydomain "github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
	"github.com/gradionhq/margince/backend/internal/platform/database"
)

const (
	defaultFieldHistoryLimit = 50
	maxFieldHistoryLimit     = 200
	fieldHistoryScanBatch    = 100
	fieldHistoryMaxScanRows  = 2000
)

// ErrInvalidCursor is returned by List when cursor is non-empty but malformed.
var ErrInvalidCursor = errors.New("records/fieldhistory: invalid cursor")

// Store queries audit_log for per-field change history (RD-WIRE-5).
type Store struct {
	db         *sql.DB
	fieldMasks map[string]audithistorydomain.EntityFieldMask
}

// NewStore returns a Store backed by db using the default field masks.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, fieldMasks: audithistorydomain.DefaultFieldMasks}
}

// WithFieldMasks returns a copy with the given masks injected — the RD-AC-5/RD-PARAM-6 test seam.
func (s *Store) WithFieldMasks(masks map[string]audithistorydomain.EntityFieldMask) *Store {
	return &Store{db: s.db, fieldMasks: masks}
}

// listState carries the mutable accumulator threaded across batch scans.
type listState struct {
	cursorTime time.Time
	cursorID   string
	useCursor  bool
	scanned    int
	entries    []Entry
}

// scanBatch drains rows into state, respecting the actorType/field filters and limit.
// Returns (batchCount, earlyStop, err). The rows are always closed before return.
func (s *Store) scanBatch(
	rows *sql.Rows,
	st *listState,
	entityType, entityID string,
	field, actorType *string,
	mask audithistorydomain.EntityFieldMask,
	limit int,
) (batchCount int, earlyStop bool, err error) {
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			rowID         string
			rowActorType  string
			rowActorID    string
			rowPassport   sql.NullString
			evidenceJSON  []byte
			rowOccurredAt time.Time
			beforeJSON    []byte
			afterJSON     []byte
		)
		if err = rows.Scan(
			&rowID, &rowActorType, &rowActorID, &rowPassport, &evidenceJSON,
			&rowOccurredAt, &beforeJSON, &afterJSON,
		); err != nil {
			return batchCount, false, err
		}
		batchCount++
		st.scanned++
		st.cursorTime = rowOccurredAt
		st.cursorID = rowID
		st.useCursor = true

		if actorType != nil && rowActorType != *actorType {
			if st.scanned >= fieldHistoryMaxScanRows {
				return batchCount, true, nil
			}
			continue
		}

		rowEntries := diffRowFields(unmarshalRow(auditLogRow{
			id:         rowID,
			entityType: entityType,
			entityID:   entityID,
			actorType:  rowActorType,
			actorID:    rowActorID,
			occurredAt: rowOccurredAt,
		}, rowPassport, evidenceJSON, beforeJSON, afterJSON), mask, field)
		st.entries = append(st.entries, rowEntries...)

		if len(st.entries) >= limit || st.scanned >= fieldHistoryMaxScanRows {
			return batchCount, true, nil
		}
	}
	return batchCount, false, rows.Err()
}

// unmarshalRow fills the JSON-backed fields of an auditLogRow from the raw DB bytes.
func unmarshalRow(
	base auditLogRow,
	passport sql.NullString,
	evidenceJSON, beforeJSON, afterJSON []byte,
) auditLogRow {
	if passport.Valid {
		base.passportID = &passport.String
	}
	if len(evidenceJSON) > 0 {
		_ = json.Unmarshal(evidenceJSON, &base.evidence)
	}
	if len(beforeJSON) > 0 {
		_ = json.Unmarshal(beforeJSON, &base.before)
	}
	if len(afterJSON) > 0 {
		_ = json.Unmarshal(afterJSON, &base.after)
	}
	return base
}

// List returns a cursor-paginated page of field-history entries for entityType/entityID,
// optionally narrowed by field/actorType, newest first. Always returns a non-nil slice
// (possibly empty) and never errs.ErrNotFound — a nonexistent id or zero-match filter is an
// honest empty page (RD-AC-5). Returns ErrInvalidCursor when cursor is non-empty and malformed.
func (s *Store) List(
	ctx context.Context,
	workspaceID, entityType, entityID string,
	field, actorType *string,
	cursor string,
	limit int,
) ([]Entry, string, error) {
	if limit <= 0 {
		limit = defaultFieldHistoryLimit
	}
	if limit > maxFieldHistoryLimit {
		limit = maxFieldHistoryLimit
	}

	afterTime, afterID, ok := decodeCursor(cursor)
	if !ok {
		return nil, "", ErrInvalidCursor
	}

	mask := s.fieldMasks[entityType]
	st := listState{
		cursorTime: afterTime,
		cursorID:   afterID,
		useCursor:  !afterTime.IsZero(),
	}

	var nextCursor string

	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		for {
			rows, err := s.queryBatch(ctx, tx, workspaceID, entityType, entityID, st.cursorTime, st.cursorID, st.useCursor)
			if err != nil {
				return err
			}

			batchCount, earlyStop, err := s.scanBatch(rows, &st, entityType, entityID, field, actorType, mask, limit)
			if err != nil {
				return err
			}

			if earlyStop {
				nextCursor = encodeCursor(st.cursorTime, st.cursorID)
				return nil
			}
			if batchCount < fieldHistoryScanBatch {
				nextCursor = ""
				return nil
			}
		}
	})
	if err != nil {
		return nil, "", err
	}
	if st.entries == nil {
		st.entries = []Entry{}
	}
	return st.entries, nextCursor, nil
}

// queryBatch fetches up to fieldHistoryScanBatch audit_log rows for the entity, ordered newest first.
// When useCursor is true the keyset predicate (occurred_at, id) < (cursorTime, cursorID) is applied.
func (s *Store) queryBatch(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID, entityType, entityID string,
	cursorTime time.Time, cursorID string,
	useCursor bool,
) (*sql.Rows, error) {
	if useCursor {
		return tx.QueryContext(ctx, `
			SELECT id, actor_type, actor_id, passport_id, evidence, occurred_at, before, after
			FROM audit_log
			WHERE workspace_id = $1::uuid AND entity_type = $2 AND entity_id = $3::uuid
			  AND (occurred_at, id) < ($4, $5::uuid)
			ORDER BY occurred_at DESC, id DESC
			LIMIT $6`,
			workspaceID, entityType, entityID, cursorTime, cursorID, fieldHistoryScanBatch)
	}
	return tx.QueryContext(ctx, `
		SELECT id, actor_type, actor_id, passport_id, evidence, occurred_at, before, after
		FROM audit_log
		WHERE workspace_id = $1::uuid AND entity_type = $2 AND entity_id = $3::uuid
		ORDER BY occurred_at DESC, id DESC
		LIMIT $4`,
		workspaceID, entityType, entityID, fieldHistoryScanBatch)
}
