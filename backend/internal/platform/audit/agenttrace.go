package crmaudit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// ToolCall is one ordered tool invocation in a trace.
type ToolCall struct {
	Name string `json:"name"`
	Args any    `json:"args"`
}

// TraceEntry is a producer-agnostic agent-action trace. It does not require
// the Surface-B runner; any producer (or a recorded fixture) can ingest one.
type TraceEntry struct {
	WorkspaceID   string
	TraceID       string
	ActorID       string
	AuditLogID    *string
	Inputs        any
	ToolCalls     []ToolCall
	Outputs       any
	ApprovalState string
}

// Ingest records one agent_trace row and returns its id.
func Ingest(ctx context.Context, db *sql.DB, te TraceEntry) (string, error) {
	if te.WorkspaceID == "" || te.TraceID == "" {
		return "", fmt.Errorf("crmaudit ingest: workspace_id and trace_id required")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id',$1,true)`, te.WorkspaceID); err != nil {
		return "", err
	}
	calls, _ := json.Marshal(te.ToolCalls)
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agent_trace (workspace_id, trace_id, audit_log_id, actor_id, inputs, tool_calls, outputs, approval_state)
		VALUES ($1::uuid,$2,$3::uuid,$4,$5,$6,$7,$8) RETURNING id`,
		te.WorkspaceID, te.TraceID, te.AuditLogID, te.ActorID,
		jsonOrNil(te.Inputs), calls, jsonOrNil(te.Outputs), nullStr(te.ApprovalState)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("crmaudit ingest insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// ResolveFromAudit returns the most recent trace linked to an audit row.
func ResolveFromAudit(ctx context.Context, db *sql.DB, auditLogID string) (TraceEntry, error) {
	var te TraceEntry
	var calls []byte
	var inputs, outputs []byte
	var approval sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT workspace_id::text, trace_id, audit_log_id::text, actor_id, inputs, tool_calls, outputs, approval_state
		FROM agent_trace WHERE audit_log_id=$1::uuid ORDER BY created_at DESC LIMIT 1`,
		auditLogID).Scan(&te.WorkspaceID, &te.TraceID, &te.AuditLogID, &te.ActorID, &inputs, &calls, &outputs, &approval)
	if err != nil {
		return TraceEntry{}, fmt.Errorf("crmaudit resolve: %w", err)
	}
	_ = json.Unmarshal(calls, &te.ToolCalls)
	if len(inputs) > 0 {
		_ = json.Unmarshal(inputs, &te.Inputs)
	}
	if len(outputs) > 0 {
		_ = json.Unmarshal(outputs, &te.Outputs)
	}
	te.ApprovalState = approval.String
	return te, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
