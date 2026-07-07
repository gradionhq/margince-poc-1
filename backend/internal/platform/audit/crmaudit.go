// Package crmaudit is the append-only audit-log seam: every mutation writes one
// audit_log row in the same tx as the change, plus the agent-trace and
// manual-entry-smell projections over that log.
package crmaudit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// Entry is one audited mutation. Before/After are marshaled to jsonb.
type Entry struct {
	WorkspaceID       string
	ActorType         string // human | agent | system | connector
	ActorID           string
	PassportID        *string // set for agent actions when known; NULL otherwise
	OnBehalfOf        *string // granting human (uuid) for agent/connector actions
	Action            string  // create | update | archive | merge | promote | restore | export | erase | login | assign | advance_stage | capture
	EntityType        string
	EntityID          *string
	Before            any
	After             any
	AuthorizationRule *string
	Evidence          any // optional structured context (e.g. idempotency key), stored in audit_log.evidence
}

// EntryFromPrincipal shapes attribution from the ctx Principal:
//   - human  -> ActorType=human, ActorID=UserID, PassportID=nil
//   - agent  -> ActorType=agent, ActorID=UserID, OnBehalfOf=UserID (granting human)
//   - none   -> ActorType=system, ActorID=system
func EntryFromPrincipal(ctx context.Context, action, entityType string, entityID *string, before, after any) Entry {
	e := Entry{
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Before:     before,
		After:      after,
	}
	p, ok := crmctx.From(ctx)
	if !ok || p.UserID == "" {
		e.ActorType, e.ActorID = "system", "system"
		return e
	}
	e.WorkspaceID = p.TenantID
	e.ActorID = p.UserID
	if p.IsAgent {
		e.ActorType = "agent"
		obo := p.UserID
		e.OnBehalfOf = &obo
		return e
	}
	e.ActorType = "human"
	return e
}

func jsonOrNil(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// DBExec is satisfied by both *sql.Tx and *sql.DB.
type DBExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// WriteTx appends exactly one audit_log row using the caller's tx (so the
// audit row is atomic with the mutation) and returns its id. LOUD: any error
// is returned, never swallowed. (B-EP07.4 adds the audit.appended outbox enqueue
// in Task 3.)
func WriteTx(ctx context.Context, tx DBExec, e Entry) (string, error) {
	if e.WorkspaceID == "" {
		if p, ok := crmctx.From(ctx); ok {
			e.WorkspaceID = p.TenantID
		}
	}
	if e.WorkspaceID == "" {
		return "", fmt.Errorf("crmaudit: empty workspace_id")
	}
	var auditID string
	err := tx.QueryRowContext(ctx, `
		INSERT INTO audit_log
		  (workspace_id, actor_type, actor_id, passport_id, on_behalf_of,
		   action, entity_type, entity_id, before, after, authorization_rule, evidence)
		VALUES ($1::uuid,$2,$3,$4::uuid,$5::uuid,$6,$7,$8::uuid,$9,$10,$11,$12)
		RETURNING id`,
		e.WorkspaceID, e.ActorType, e.ActorID, e.PassportID, e.OnBehalfOf,
		e.Action, e.EntityType, e.EntityID, jsonOrNil(e.Before), jsonOrNil(e.After),
		e.AuthorizationRule, jsonOrNil(e.Evidence)).Scan(&auditID)
	if err != nil {
		return "", fmt.Errorf("crmaudit insert: %w", err)
	}
	if err := EmitAuditAppended(ctx, tx, e, auditID); err != nil {
		return "", err
	}
	return auditID, nil
}

// EmitAuditAppended enqueues one audit.appended outbox event for an audit row,
// idempotent on audit_log_id (the event's entity_id). Called by WriteTx in the
// audit-row tx, and by the relay to re-deliver (at-least-once).
func EmitAuditAppended(ctx context.Context, tx DBExec, e Entry, auditLogID string) error {
	payload, _ := json.Marshal(map[string]any{
		"audit_log_id":       auditLogID,
		"action":             e.Action,
		"entity_type":        e.EntityType,
		"entity_id":          e.EntityID,
		"authorization_rule": e.AuthorizationRule,
	})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO event_outbox (workspace_id, topic, entity_id, payload)
		VALUES ($1::uuid,'audit.appended',$2::uuid,$3)
		ON CONFLICT (entity_id) WHERE topic='audit.appended' DO NOTHING`,
		e.WorkspaceID, auditLogID, payload); err != nil {
		return fmt.Errorf("crmaudit emit: %w", err)
	}
	return nil
}

// Write is the convenience for non-transactional call sites: it opens its own
// short tx, sets the RLS GUC, writes the row, and commits. LOUD.
func Write(ctx context.Context, db *sql.DB, e Entry) (string, error) {
	if e.WorkspaceID == "" {
		if p, ok := crmctx.From(ctx); ok {
			e.WorkspaceID = p.TenantID
		}
	}
	if e.WorkspaceID == "" {
		return "", fmt.Errorf("crmaudit: empty workspace_id")
	}
	var auditID string
	err := database.WithWorkspaceTx(ctx, db, e.WorkspaceID, func(tx *sql.Tx) error {
		var err error
		auditID, err = WriteTx(ctx, tx, e)
		return err
	})
	if err != nil {
		return "", err
	}
	return auditID, nil
}
