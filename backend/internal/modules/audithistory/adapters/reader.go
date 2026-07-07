// Package adapters contains the database-backed implementations of the audithistory ports.
package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// AuditHistoryReader queries audit_log for a given entity and renders history lines.
type AuditHistoryReader struct {
	db         *sql.DB
	fieldMasks map[string]domain.EntityFieldMask // injected for testability; defaults to domain.DefaultFieldMasks
}

// NewAuditHistoryReader returns a reader using the default field masks.
func NewAuditHistoryReader(db *sql.DB) *AuditHistoryReader {
	return &AuditHistoryReader{db: db, fieldMasks: domain.DefaultFieldMasks}
}

// WithFieldMasks returns a copy of the reader with the given field masks injected.
// Intended for integration tests that need to assert masking behaviour.
func (r *AuditHistoryReader) WithFieldMasks(masks map[string]domain.EntityFieldMask) *AuditHistoryReader {
	return &AuditHistoryReader{db: r.db, fieldMasks: masks}
}

// ReadHistory returns all audit_log rows for the given entity, ordered chronologically,
// with before/after masked to the viewer's readable fields.
// The caller MUST have already set the RLS GUC (app.workspace_id) on the connection
// via the standard set_config pattern.
func (r *AuditHistoryReader) ReadHistory(ctx context.Context, entityType, entityID, workspaceID string) ([]domain.AuditHistoryEntry, error) {
	// Set RLS GUC for this query, consistent with the crm-audit pattern.
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("audit history begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := database.SetWorkspaceScope(ctx, tx, workspaceID); err != nil {
		return nil, fmt.Errorf("audit history set guc: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT
			a.id, a.actor_type, a.actor_id, a.on_behalf_of, a.action,
			a.occurred_at, a.authorization_rule, a.before, a.after,
			obo.display_name AS obo_name,
			actor.display_name AS actor_name
		FROM audit_log a
		LEFT JOIN app_user obo ON obo.id = a.on_behalf_of
		LEFT JOIN app_user actor ON actor.id = CASE WHEN a.actor_id ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN a.actor_id::uuid ELSE NULL END
		WHERE a.entity_type = $1 AND a.entity_id = $2::uuid AND a.workspace_id = $3::uuid
		ORDER BY a.occurred_at ASC, a.id ASC`,
		entityType, entityID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("audit history query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	mask := r.fieldMasks[entityType]
	var entries []domain.AuditHistoryEntry
	for rows.Next() {
		e, err := scanHistoryRow(rows, mask)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit history rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("audit history commit: %w", err)
	}
	return entries, nil
}

// scanHistoryRow scans one audit_log row, masks before/after to the viewer's
// readable fields, and composes the human-readable summary.
func scanHistoryRow(rows *sql.Rows, mask domain.EntityFieldMask) (domain.AuditHistoryEntry, error) {
	var (
		e          domain.AuditHistoryEntry
		onBehalf   sql.NullString
		oboName    sql.NullString
		actorName  sql.NullString
		authRule   sql.NullString
		beforeJSON []byte
		afterJSON  []byte
	)
	if err := rows.Scan(
		&e.ID, &e.ActorType, &e.ActorID, &onBehalf,
		&e.Action, &e.OccurredAt, &authRule, &beforeJSON, &afterJSON, &oboName, &actorName,
	); err != nil {
		return domain.AuditHistoryEntry{}, fmt.Errorf("audit history scan: %w", err)
	}
	if onBehalf.Valid {
		e.OnBehalfOf = &onBehalf.String
	}
	var oboNamePtr *string
	if oboName.Valid {
		e.OnBehalfOfName = &oboName.String
		oboNamePtr = &oboName.String
	}
	if authRule.Valid {
		e.AuthorizationRule = &authRule.String
	}

	var beforeMap, afterMap map[string]any
	if len(beforeJSON) > 0 {
		_ = json.Unmarshal(beforeJSON, &beforeMap)
	}
	if len(afterJSON) > 0 {
		_ = json.Unmarshal(afterJSON, &afterMap)
	}
	e.Before = domain.ApplyFieldMask(beforeMap, mask)
	e.After = domain.ApplyFieldMask(afterMap, mask)

	// Resolve actor display name for summary: use app_user.display_name when available,
	// falling back to actor_id (UUID or "system").
	actorDisplay := e.ActorID
	if actorName.Valid && actorName.String != "" {
		actorDisplay = actorName.String
	}
	e.Summary = domain.ComposeSummary(e.ActorType, actorDisplay, oboNamePtr, e.Action)
	return e, nil
}
