package adapters

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/gradionhq/margince/backend/internal/modules/gdpr/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// RetentionWorker evaluates all retention policies across workspaces and applies
// the archive/anonymize/erase ladder, skipping records with legal_hold=true.
type RetentionWorker struct {
	river.WorkerDefaults[domain.RetentionSweepArgs]
	db *sql.DB
}

// NewRetentionWorker returns a RetentionWorker backed by the given db.
func NewRetentionWorker(db *sql.DB) *RetentionWorker { return &RetentionWorker{db: db} }

// isUnconverted reports whether a lead has not been promoted.
func isUnconverted(status string) bool { return status != "promoted" }

// isLostDeal reports whether a deal is in the lost status.
func isLostDeal(status string) bool { return status == statusLost }

// isTranscript reports whether an activity is a transcript (source_system='transcript').
func isTranscript(sourceSystem string) bool { return sourceSystem == sourceTranscript }

// nonPersonEraseSupported reports whether the non-person erase action has a real
// implementation for the given object type. Only activities are erasable on this path.
func nonPersonEraseSupported(objectType string) bool { return objectType == objectActivity }

// workItem pairs a record ID with the policy that applies to it.
type workItem struct {
	id     string
	policy domain.Policy
}

// Work runs the nightly retention sweep across all workspaces.
func (w *RetentionWorker) Work(ctx context.Context, job *river.Job[domain.RetentionSweepArgs]) error {
	now := time.Now().UTC()

	wsIDs, err := w.allWorkspaceIDs(ctx)
	if err != nil {
		return fmt.Errorf("retention sweep: list workspaces: %w", err)
	}

	for _, wsID := range wsIDs {
		if err := w.processWorkspace(ctx, wsID, now); err != nil {
			return fmt.Errorf("retention sweep workspace %s: %w", wsID, err)
		}
	}
	return nil
}

// allWorkspaceIDs returns all workspace IDs.
func (w *RetentionWorker) allWorkspaceIDs(ctx context.Context) ([]string, error) {
	// rls-exempt: workspace table has no RLS (migration 000002) — no workspace_id to scope by; this IS the workspace list.
	rows, err := w.db.QueryContext(ctx, `SELECT id FROM workspace`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// processWorkspace collects all over-age work items for a workspace (phase 1),
// then applies each action in its own transaction (phase 2).
func (w *RetentionWorker) processWorkspace(ctx context.Context, wsID string, now time.Time) error {
	items, err := w.collectWorkItems(ctx, wsID, now)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := w.applyAction(ctx, wsID, item.policy, item.id); err != nil {
			return err
		}
	}
	return nil
}

// collectWorkItems opens a read-only transaction, sets the workspace GUC, loads
// all enabled policies, and returns all over-age record IDs with their policies.
func (w *RetentionWorker) collectWorkItems(ctx context.Context, wsID string, now time.Time) ([]workItem, error) {
	var items []workItem
	err := database.WithWorkspaceTx(ctx, w.db, wsID, func(tx *sql.Tx) error {
		policies, err := loadPoliciesTx(ctx, tx, wsID)
		if err != nil {
			return err
		}
		for _, p := range policies {
			cutoff := now.Add(-time.Duration(p.RetainDays) * 24 * time.Hour)
			ids, err := selectOverAgeQuery(ctx, tx, wsID, p, cutoff)
			if err != nil {
				return err
			}
			for _, id := range ids {
				items = append(items, workItem{id: id, policy: p})
			}
		}
		return nil
	})
	return items, err
}

// loadPoliciesTx returns all enabled retention policies for the given workspace.
func loadPoliciesTx(ctx context.Context, tx *sql.Tx, wsID string) ([]domain.Policy, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT object_type, COALESCE(category,''), retain_days, action
		 FROM retention_policy WHERE workspace_id = $1::uuid AND enabled = true`, wsID)
	if err != nil {
		return nil, fmt.Errorf("loadPolicies query: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var policies []domain.Policy
	for rows.Next() {
		var p domain.Policy
		if err := rows.Scan(&p.ObjectType, &p.Category, &p.RetainDays, &p.Action); err != nil {
			return nil, fmt.Errorf("loadPolicies scan: %w", err)
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

// selectOverAgeQuery returns IDs of records that match the policy's category filter and
// are older than the cutoff.
func selectOverAgeQuery(ctx context.Context, tx *sql.Tx, wsID string, p domain.Policy, cutoff time.Time) ([]string, error) {
	key := p.ObjectType + "/" + p.Category
	var query string
	switch key {
	case "lead/unconverted":
		query = `SELECT id::text FROM lead
		         WHERE workspace_id=$1::uuid AND status != 'promoted'
		           AND legal_hold = false AND archived_at IS NULL AND updated_at < $2`
	case "activity/":
		query = `SELECT id::text FROM activity
		         WHERE workspace_id=$1::uuid AND archived_at IS NULL AND updated_at < $2`
	case "activity/transcript":
		query = `SELECT id::text FROM activity
		         WHERE workspace_id=$1::uuid AND source_system = 'transcript'
		           AND archived_at IS NULL AND updated_at < $2`
	case "person/no_consent_no_deal":
		query = `SELECT p.id::text FROM person p
		         WHERE p.workspace_id=$1::uuid AND p.legal_hold = false
		           AND p.archived_at IS NULL AND p.updated_at < $2
		           AND NOT EXISTS (
		             SELECT 1 FROM person_consent pc
		             WHERE pc.person_id = p.id AND pc.workspace_id = p.workspace_id
		               AND pc.state = 'granted'
		           )
		           AND NOT EXISTS (
		             SELECT 1 FROM activity_link al1
		             JOIN activity_link al2
		               ON al2.activity_id = al1.activity_id AND al2.entity_type = 'deal'
		             WHERE al1.person_id = p.id
		           )`
	case "deal/lost":
		query = `SELECT id::text FROM deal
		         WHERE workspace_id=$1::uuid AND status = 'lost'
		           AND legal_hold = false AND archived_at IS NULL AND updated_at < $2`
	default:
		return nil, nil
	}

	rows, err := tx.QueryContext(ctx, query, wsID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("selectOverAgeIDs %s: %w", key, err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("selectOverAgeIDs scan %s: %w", key, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("selectOverAgeIDs rows %s: %w", key, err)
	}
	return ids, nil
}

// applyAction applies the ladder action for one record and writes an audit row.
func (w *RetentionWorker) applyAction(ctx context.Context, wsID string, p domain.Policy, recordID string) error {
	if p.Action == actionErase && p.ObjectType == objectPerson {
		eraseCtx := crmctx.With(ctx, crmctx.Principal{TenantID: wsID})
		return Erase(eraseCtx, w.db, recordID)
	}

	return database.WithWorkspaceTx(ctx, w.db, wsID, func(tx *sql.Tx) error {
		auditAction, err := runRetentionAction(ctx, tx, wsID, p, recordID)
		if err != nil {
			return err
		}

		entry := crmaudit.Entry{
			WorkspaceID: wsID, ActorType: actorSystem, ActorID: actorSystem,
			Action: auditAction, EntityType: p.ObjectType, EntityID: &recordID,
		}
		if _, err := crmaudit.WriteTx(ctx, tx, entry); err != nil {
			return fmt.Errorf("applyAction audit: %w", err)
		}
		return nil
	})
}

// runRetentionAction applies the ladder action for one record within the
// caller's transaction and returns the audit action string to record.
func runRetentionAction(ctx context.Context, tx *sql.Tx, wsID string, p domain.Policy, recordID string) (string, error) {
	switch p.Action {
	case actionArchive:
		if err := applyArchive(ctx, tx, p.ObjectType, recordID); err != nil {
			return "", err
		}
		return actionArchive, nil
	case actionAnonymize:
		if err := applyAnonymize(ctx, tx, wsID, p.ObjectType, recordID); err != nil {
			return "", err
		}
		return "update", nil
	case actionErase:
		if !nonPersonEraseSupported(p.ObjectType) {
			return "", fmt.Errorf("applyAction: erase action unsupported for object_type %q (only person and activity are erasable)", p.ObjectType)
		}
		if err := applyActivityErase(ctx, tx, recordID); err != nil {
			return "", err
		}
		return actionErase, nil
	default:
		return "", fmt.Errorf("applyAction: unknown action %q", p.Action)
	}
}

// applyArchive sets archived_at=now() on the given record.
func applyArchive(ctx context.Context, tx *sql.Tx, objectType, recordID string) error {
	var q string
	switch objectType {
	case objectLead:
		q = `UPDATE lead SET archived_at = now() WHERE id = $1::uuid`
	case objectDeal:
		q = `UPDATE deal SET archived_at = now() WHERE id = $1::uuid`
	case objectPerson:
		q = `UPDATE person SET archived_at = now() WHERE id = $1::uuid`
	case "organization":
		q = `UPDATE organization SET archived_at = now() WHERE id = $1::uuid`
	case objectActivity:
		q = `UPDATE activity SET archived_at = now() WHERE id = $1::uuid`
	default:
		return fmt.Errorf("applyArchive: unsupported object_type %q", objectType)
	}
	if _, err := tx.ExecContext(ctx, q, recordID); err != nil {
		return fmt.Errorf("applyArchive %s: %w", objectType, err)
	}
	return nil
}

// applyAnonymize nulls PII fields for the record.
func applyAnonymize(ctx context.Context, tx *sql.Tx, wsID, objectType, recordID string) error {
	switch objectType {
	case objectLead:
		_, err := tx.ExecContext(ctx, `
			UPDATE lead SET full_name=NULL, email=NULL, title=NULL,
			                company_name=NULL, candidate_org_key=NULL, raw=NULL
			WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			recordID, wsID)
		if err != nil {
			return fmt.Errorf("applyAnonymize lead: %w", err)
		}
	case objectPerson:
		_, err := tx.ExecContext(ctx, `
			UPDATE person SET first_name=NULL, last_name=NULL, full_name='[anonymized]',
			                  title=NULL, social='{}'::jsonb, address=NULL, raw=NULL
			WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			recordID, wsID)
		if err != nil {
			return fmt.Errorf("applyAnonymize person: %w", err)
		}
	default:
		return fmt.Errorf("applyAnonymize: unsupported object_type %q", objectType)
	}
	return nil
}

// applyActivityErase nulls PII fields on an activity and sets archived_at.
func applyActivityErase(ctx context.Context, tx *sql.Tx, recordID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE activity SET subject=NULL, body=NULL, raw=NULL, archived_at=now()
		WHERE id=$1::uuid`,
		recordID)
	if err != nil {
		return fmt.Errorf("applyActivityErase: %w", err)
	}
	return nil
}
