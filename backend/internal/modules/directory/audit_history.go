package crmcore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// AuditHistoryEntry is one rendered history line for a record mutation.
// before/after are field-masked to the viewer's readable fields.
type AuditHistoryEntry struct {
	ID                string         `json:"id"`
	ActorType         string         `json:"actor_type"`
	ActorID           string         `json:"actor_id"`
	OnBehalfOf        *string        `json:"on_behalf_of,omitempty"`
	OnBehalfOfName    *string        `json:"on_behalf_of_name,omitempty"`
	Action            string         `json:"action"`
	OccurredAt        time.Time      `json:"occurred_at"`
	AuthorizationRule *string        `json:"authorization_rule,omitempty"`
	Before            map[string]any `json:"before,omitempty"`
	After             map[string]any `json:"after,omitempty"`
	Summary           string         `json:"summary"`
}

// EntityFieldMask is the set of field names to hide in before/after for an entity type.
// An absent field in the mask is visible; a present one is removed from the output.
type EntityFieldMask map[string]struct{}

// DefaultFieldMasks is the minimal per-entity-type mask required by AC2.
// Extend here when a future story adds field-level RBAC columns.
// An absent entry (or nil mask) means no fields are masked.
var DefaultFieldMasks = map[string]EntityFieldMask{
	// No fields are masked by default in V1; the seam is here for AC2 tests
	// to inject a non-empty mask. Future stories populate this map.
}

// applyFieldMask returns a copy of data with every key in mask removed.
// Returns nil when data is nil.
func applyFieldMask(data map[string]any, mask EntityFieldMask) map[string]any {
	if data == nil {
		return nil
	}
	if len(mask) == 0 {
		out := make(map[string]any, len(data))
		for k, v := range data {
			out[k] = v
		}
		return out
	}
	out := make(map[string]any, len(data))
	for k, v := range data {
		if _, hidden := mask[k]; !hidden {
			out[k] = v
		}
	}
	return out
}

// composeSummary builds a plain-language description of one audit event.
// actorDisplayName is the human-readable name of the actor (e.g. display_name from app_user,
// or actor_id when no name is available). onBehalfOfName is set only for agent actions.
func composeSummary(actorType, actorDisplayName string, onBehalfOfName *string, action string) string {
	actionWord := actionToVerb(action)
	switch actorType {
	case actorTypeAgent:
		if onBehalfOfName != nil && *onBehalfOfName != "" {
			return fmt.Sprintf("Agent acting for %s %s the record", *onBehalfOfName, actionWord)
		}
		return fmt.Sprintf("Agent %s the record", actionWord)
	case "human":
		return fmt.Sprintf("%s %s the record", actorDisplayName, actionWord)
	case "system":
		return fmt.Sprintf("System %s the record", actionWord)
	case actorTypeConnector:
		return fmt.Sprintf("Connector %s the record", actionWord)
	default:
		return fmt.Sprintf("%s %s the record", actorType, actionWord)
	}
}

// actionVerbs maps an audit action to its past-tense verb phrase. Unknown actions
// fall back to the raw action string (see actionToVerb).
var actionVerbs = map[string]string{
	"create":           "created",
	"update":           "updated",
	"archive":          "archived",
	"merge":            "merged",
	"promote":          actionPromoted,
	"disqualify":       "disqualified",
	"restore":          "restored",
	"export":           "exported",
	"erase":            "erased",
	"anonymize":        "anonymized",
	"login":            "logged in",
	"assign":           "assigned",
	"advance_stage":    "advanced the stage of",
	"send_email":       "sent an email for",
	"consent_grant":    "granted consent for",
	"consent_withdraw": "withdrew consent for",
	"approve":          actionApproved,
	"reject":           "rejected",
	"record_share":     "shared",
	"record_unshare":   "unshared",
}

// actionToVerb converts an audit action to a past-tense verb phrase.
func actionToVerb(action string) string {
	if verb, ok := actionVerbs[action]; ok {
		return verb
	}
	return action
}

// AuditHistoryReader queries audit_log for a given entity and renders history lines.
type AuditHistoryReader struct {
	db         *sql.DB
	fieldMasks map[string]EntityFieldMask // injected for testability; defaults to DefaultFieldMasks
}

// NewAuditHistoryReader returns a reader using the default field masks.
func NewAuditHistoryReader(db *sql.DB) *AuditHistoryReader {
	return &AuditHistoryReader{db: db, fieldMasks: DefaultFieldMasks}
}

// withFieldMasks returns a copy of the reader with the given masks (for testing).
func (r *AuditHistoryReader) withFieldMasks(masks map[string]EntityFieldMask) *AuditHistoryReader {
	return &AuditHistoryReader{db: r.db, fieldMasks: masks}
}

// ReadHistory returns all audit_log rows for the given entity, ordered chronologically,
// with before/after masked to the viewer's readable fields.
// The caller MUST have already set the RLS GUC (app.workspace_id) on the connection
// via the standard set_config pattern.
func (r *AuditHistoryReader) ReadHistory(ctx context.Context, entityType, entityID, workspaceID string) ([]AuditHistoryEntry, error) {
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
	var entries []AuditHistoryEntry
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
func scanHistoryRow(rows *sql.Rows, mask EntityFieldMask) (AuditHistoryEntry, error) {
	var (
		e          AuditHistoryEntry
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
		return AuditHistoryEntry{}, fmt.Errorf("audit history scan: %w", err)
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
	e.Before = applyFieldMask(beforeMap, mask)
	e.After = applyFieldMask(afterMap, mask)

	// Resolve actor display name for summary: use app_user.display_name when available,
	// falling back to actor_id (UUID or "system").
	actorDisplay := e.ActorID
	if actorName.Valid && actorName.String != "" {
		actorDisplay = actorName.String
	}
	e.Summary = composeSummary(e.ActorType, actorDisplay, oboNamePtr, e.Action)
	return e, nil
}
