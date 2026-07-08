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

// earlyStopReason distinguishes why scanBatch stopped mid-batch, since the two reasons
// warrant different has_more handling in List:
//   - earlyStopPageFull: the requested page is exactly full (len(entries) >= limit) after a
//     whole audit_log row's entries were appended (row-boundary preservation — a row's
//     entries are never split across pages). This does NOT by itself prove another row
//     exists past the cursor; List must check.
//   - earlyStopScanCap: the defensive fieldHistoryMaxScanRows cap was hit. The loop stopped
//     without exhausting the underlying data and there's no cheap way to know whether more
//     rows exist beyond the cap, so reporting has_more=true unconditionally is correct here.
type earlyStopReason int

const (
	noEarlyStop earlyStopReason = iota
	earlyStopPageFull
	earlyStopScanCap
)

// scanBatch drains rows into state, respecting the actorType/field filters and limit.
// Returns (batchCount, stop, err). The rows are always closed before return.
func (s *Store) scanBatch(
	rows *sql.Rows,
	st *listState,
	entityType, entityID string,
	field, actorType *string,
	mask audithistorydomain.EntityFieldMask,
	limit int,
) (batchCount int, stop earlyStopReason, err error) {
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
			return batchCount, noEarlyStop, err
		}
		batchCount++
		st.scanned++
		st.cursorTime = rowOccurredAt
		st.cursorID = rowID
		st.useCursor = true

		if actorType != nil && rowActorType != *actorType {
			if st.scanned >= fieldHistoryMaxScanRows {
				return batchCount, earlyStopScanCap, nil
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

		// Scan-cap takes priority when both trigger on the same row: it's always safe to
		// report has_more=true unconditionally (see earlyStopReason doc), so there's no
		// need to pay for the extra existence check below.
		if st.scanned >= fieldHistoryMaxScanRows {
			return batchCount, earlyStopScanCap, nil
		}
		if len(st.entries) >= limit {
			return batchCount, earlyStopPageFull, nil
		}
	}
	return batchCount, noEarlyStop, rows.Err()
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

			batchCount, stop, err := s.scanBatch(rows, &st, entityType, entityID, field, actorType, mask, limit)
			if err != nil {
				return err
			}

			switch stop {
			case earlyStopScanCap:
				// The loop stopped without exhausting the underlying data — no cheap way
				// to know whether more rows exist beyond the cap, so has_more=true is the
				// correct, honest answer here.
				nextCursor = encodeCursor(st.cursorTime, st.cursorID)
				return nil
			case earlyStopPageFull:
				// The requested page is exactly full, but that alone doesn't prove another
				// row exists past the cursor — the row that filled the page may be the
				// true last audit_log row for this entity (RD-T07 UAT fix). Disambiguate
				// with one cheap existence check before claiming has_more=true.
				more, err := s.hasFollowingRow(ctx, tx, workspaceID, entityType, entityID, st.cursorTime, st.cursorID)
				if err != nil {
					return err
				}
				if more {
					nextCursor = encodeCursor(st.cursorTime, st.cursorID)
				} else {
					nextCursor = ""
				}
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

// hasFollowingRow reports whether at least one audit_log row exists strictly past the
// (cursorTime, cursorID) keyset position for this entity_type/entity_id — the cheap
// follow-up check used only to disambiguate earlyStopPageFull from genuine exhaustion
// (RD-T07 UAT fix). The predicate mirrors queryBatch's cursor branch, minus the LIMIT,
// so it is served by the same idx_audit_entity composite index.
//
// Deliberately does NOT re-apply the actor_type filter: doing so would require carrying
// it through as a third parameter and duplicating queryBatch's filter shape for a single
// EXISTS check. Instead this accepts the same class of imprecision the earlyStopScanCap
// path already accepts unconditionally — a following row that would itself be filtered
// out by actor_type can produce a rare false-positive has_more=true (the client follows
// the cursor and gets one further, genuinely-empty page), never a false negative that
// would silently truncate real data.
func (s *Store) hasFollowingRow(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID, entityType, entityID string,
	cursorTime time.Time,
	cursorID string,
) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM audit_log
			WHERE workspace_id = $1::uuid AND entity_type = $2 AND entity_id = $3::uuid
			  AND (occurred_at, id) < ($4, $5::uuid)
		)`,
		workspaceID, entityType, entityID, cursorTime, cursorID).Scan(&exists)
	return exists, err
}
