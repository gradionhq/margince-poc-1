package crmapprovals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// ComputePreview returns a JSON snapshot of what an action would look like
// without performing any mutation. For record edits it calls datasource.Provider.Read
// to fetch the current state (before) and merges the proposed fields (after).
// For send_email it simply reflects the body and recipients from the payload.
func ComputePreview(ctx context.Context, p datasource.Provider, actionType string, payload json.RawMessage) (json.RawMessage, error) {
	switch actionType {
	case "send_email":
		return previewEmail(payload)
	default:
		return previewRecordEdit(ctx, p, payload)
	}
}

func previewEmail(payload json.RawMessage) (json.RawMessage, error) {
	var data struct {
		Body       string   `json:"body"`
		Recipients []string `json:"recipients"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("crmapprovals preview email: %w", err)
	}
	out, err := json.Marshal(map[string]any{
		"body":       data.Body,
		"recipients": data.Recipients,
	})
	if err != nil {
		return nil, fmt.Errorf("crmapprovals preview email marshal: %w", err)
	}
	return json.RawMessage(out), nil
}

func previewRecordEdit(ctx context.Context, p datasource.Provider, payload json.RawMessage) (json.RawMessage, error) {
	var data struct {
		Kind   string         `json:"kind"`
		ID     string         `json:"id"`
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("crmapprovals preview record: %w", err)
	}

	var before any
	current, err := p.Read(ctx, datasource.EntityRef{Type: datasource.EntityType(data.Kind), ID: data.ID})
	switch {
	case err == nil:
		before = current
	case errors.Is(err, errs.ErrNotFound):
		// Legitimately no prior record (a create-shaped edit): before stays nil.
	default:
		// A real Read failure (provider down, etc.) must NOT silently produce a
		// before:null preview that reads to the human as "no prior value". Surface it
		// so the gate can flag the staged item preview_unavailable (AC-MCP-4).
		return nil, fmt.Errorf("crmapprovals preview record read: %w", err)
	}

	// Build after by merging fields onto the current record.
	after := make(map[string]any)
	if m, ok := current.(map[string]any); ok {
		for k, v := range m {
			after[k] = v
		}
	}
	for k, v := range data.Fields {
		after[k] = v
	}

	out, err := json.Marshal(map[string]any{
		"before": before,
		"after":  after,
	})
	if err != nil {
		return nil, fmt.Errorf("crmapprovals preview record marshal: %w", err)
	}
	return json.RawMessage(out), nil
}
