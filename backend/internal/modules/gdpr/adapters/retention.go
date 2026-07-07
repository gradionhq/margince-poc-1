package adapters

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/gdpr/domain"
)

// defaultPolicies returns the 5 spec §3.4 seed rows (generic, no jurisdiction strings).
func defaultPolicies() []domain.Policy {
	return []domain.Policy{
		{ObjectType: objectLead, Category: "unconverted", RetainDays: 365, Action: actionAnonymize},
		{ObjectType: objectActivity, Category: "", RetainDays: 1095, Action: actionArchive},
		{ObjectType: objectActivity, Category: sourceTranscript, RetainDays: 365, Action: actionErase},
		{ObjectType: objectPerson, Category: "no_consent_no_deal", RetainDays: 730, Action: actionAnonymize},
		{ObjectType: objectDeal, Category: statusLost, RetainDays: 1825, Action: actionArchive},
	}
}

// SeedDefaults inserts the 5 default retention policies for workspaceID within tx.
// Uses ON CONFLICT DO NOTHING so it is safe to call multiple times.
func SeedDefaults(ctx context.Context, tx *sql.Tx, workspaceID string) error {
	for _, p := range defaultPolicies() {
		var cat any
		if p.Category != "" {
			cat = p.Category
		}
		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO retention_policy (workspace_id, object_type, category, retain_days, action)
			 VALUES ($1::uuid, $2, $3, $4, $5)
			 ON CONFLICT DO NOTHING`,
			workspaceID, p.ObjectType, cat, p.RetainDays, p.Action,
		)
		if err != nil {
			return fmt.Errorf("SeedDefaults %s/%s: %w", p.ObjectType, p.Category, err)
		}
	}
	return nil
}
