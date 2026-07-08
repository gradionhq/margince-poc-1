package records

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
var ErrInvalidCursor = errors.New("records: invalid cursor")

// FieldHistoryStore queries audit_log for per-field change history (RD-WIRE-5).
type FieldHistoryStore struct {
	db         *sql.DB
	fieldMasks map[string]audithistorydomain.EntityFieldMask
}

// NewFieldHistoryStore returns a FieldHistoryStore backed by db using the default field masks.
func NewFieldHistoryStore(db *sql.DB) *FieldHistoryStore {
	return &FieldHistoryStore{db: db, fieldMasks: audithistorydomain.DefaultFieldMasks}
}

// WithFieldMasks returns a copy with the given masks injected — the RD-AC-5/RD-PARAM-6 test seam,
// mirroring AuditHistoryReader.WithFieldMasks.
func (s *FieldHistoryStore) WithFieldMasks(masks map[string]audithistorydomain.EntityFieldMask) *FieldHistoryStore {
	return &FieldHistoryStore{db: s.db, fieldMasks: masks}
}

// List returns a cursor-paginated page of field-history entries for entityType/entityID,
// optionally narrowed by field/actorType, newest first. Always returns a non-nil slice
// (possibly empty) and never errs.ErrNotFound — a nonexistent id or zero-match filter is an
// honest empty page (RD-AC-5). Returns ErrInvalidCursor when cursor is non-empty and malformed.
func (s *FieldHistoryStore) List(
	ctx context.Context,
	workspaceID, entityType, entityID string,
	field, actorType *string,
	cursor string,
	limit int,
) ([]FieldHistoryEntry, string, error) {
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
	hasCursor := !afterTime.IsZero()

	mask := s.fieldMasks[entityType]

	var entries []FieldHistoryEntry
	var nextCursor string

	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		cursorTime := afterTime
		cursorID := afterID
		useCursor := hasCursor
		scanned := 0

		for {
			rows, err := s.queryBatch(ctx, tx, workspaceID, entityType, entityID, cursorTime, cursorID, useCursor)
			if err != nil {
				return err
			}

			batchCount := 0
			earlyStop := false

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
				if err := rows.Scan(
					&rowID, &rowActorType, &rowActorID, &rowPassport, &evidenceJSON,
					&rowOccurredAt, &beforeJSON, &afterJSON,
				); err != nil {
					_ = rows.Close()
					return err
				}
				batchCount++
				scanned++
				// Advance keyset to this row regardless of filtering
				cursorTime = rowOccurredAt
				cursorID = rowID
				useCursor = true

				// actor filter: skip whole row before diffing (actor is row-level, cheaper)
				if actorType != nil && rowActorType != *actorType {
					if scanned >= fieldHistoryMaxScanRows {
						earlyStop = true
						break
					}
					continue
				}

				var evidenceMap map[string]any
				if len(evidenceJSON) > 0 {
					_ = json.Unmarshal(evidenceJSON, &evidenceMap)
				}
				var passportID *string
				if rowPassport.Valid {
					passportID = &rowPassport.String
				}
				var beforeMap, afterMap map[string]any
				if len(beforeJSON) > 0 {
					_ = json.Unmarshal(beforeJSON, &beforeMap)
				}
				if len(afterJSON) > 0 {
					_ = json.Unmarshal(afterJSON, &afterMap)
				}

				rowEntries := diffRowFields(auditLogRow{
					id:         rowID,
					entityType: entityType,
					entityID:   entityID,
					actorType:  rowActorType,
					actorID:    rowActorID,
					passportID: passportID,
					evidence:   evidenceMap,
					occurredAt: rowOccurredAt,
					before:     beforeMap,
					after:      afterMap,
				}, mask, field)
				entries = append(entries, rowEntries...)

				if len(entries) >= limit || scanned >= fieldHistoryMaxScanRows {
					earlyStop = true
					break
				}
			}
			_ = rows.Close()
			if rowsErr := rows.Err(); rowsErr != nil {
				return rowsErr
			}

			if earlyStop {
				nextCursor = encodeCursor(cursorTime, cursorID)
				return nil
			}

			// Genuine exhaustion: batch was shorter than the fetch size
			if batchCount < fieldHistoryScanBatch {
				nextCursor = ""
				return nil
			}

			// Full batch with more to fetch — continue
		}
	})
	if err != nil {
		return nil, "", err
	}
	if entries == nil {
		entries = []FieldHistoryEntry{}
	}
	return entries, nextCursor, nil
}

// queryBatch fetches up to fieldHistoryScanBatch audit_log rows for the entity, ordered newest first.
// When useCursor is true the keyset predicate (occurred_at, id) < (cursorTime, cursorID) is applied.
func (s *FieldHistoryStore) queryBatch(
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
