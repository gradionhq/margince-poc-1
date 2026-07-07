package crmgdpr

import (
	"context"
	"database/sql"
	"fmt"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// buildErasureTombstone returns a PII-free crmaudit.Entry for an erasure action.
// Before and After are always nil — the tombstone must carry no personal data.
func buildErasureTombstone(personID string) crmaudit.Entry {
	pid := personID
	rule := "gdpr.erasure"
	return crmaudit.Entry{
		Action:            actionErase,
		EntityType:        objectPerson,
		EntityID:          &pid,
		Before:            nil,
		After:             nil,
		AuthorizationRule: &rule,
	}
}

// eraseNormalized irreversibly removes/anonymizes PII across normalized tables
// for the given person within the caller's transaction.
// Task 3 wires eraseRawAndVector + suppression after this step inside Erase.
func eraseNormalized(ctx context.Context, tx *sql.Tx, wsID, personID string) error {
	// 1. Null PII on person; keep id+workspace_id as tombstone anchor.
	//    social is NOT NULL, so set to empty object instead of NULL.
	if _, err := tx.ExecContext(
		ctx, `
		UPDATE person SET
		    first_name = NULL,
		    last_name  = NULL,
		    full_name  = '[erased]',
		    title      = NULL,
		    social     = '{}'::jsonb,
		    address    = NULL,
		    raw        = NULL,
		    version    = version + 1
		WHERE id = $1::uuid AND workspace_id = $2::uuid`,
		personID, wsID,
	); err != nil {
		return fmt.Errorf("eraseNormalized person: %w", err)
	}

	// 2. Delete person_email rows (email is PII).
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM person_email WHERE person_id = $1::uuid AND workspace_id = $2::uuid`,
		personID, wsID,
	); err != nil {
		return fmt.Errorf("eraseNormalized person_email: %w", err)
	}

	// 3. Null PII on the source lead if the person was converted from one.
	if _, err := tx.ExecContext(
		ctx, `
		UPDATE lead SET
		    full_name         = NULL,
		    email             = NULL,
		    title             = NULL,
		    company_name      = NULL,
		    candidate_org_key = NULL,
		    raw               = NULL
		WHERE workspace_id = $2::uuid
		  AND id = (
		    SELECT converted_from_lead_id FROM person
		    WHERE id = $1::uuid AND workspace_id = $2::uuid
		  )`,
		personID, wsID,
	); err != nil {
		return fmt.Errorf("eraseNormalized lead: %w", err)
	}

	// 4. Null PII on activities linked to this person.
	if _, err := tx.ExecContext(
		ctx, `
		UPDATE activity SET
		    subject = NULL,
		    body    = NULL,
		    raw     = NULL
		WHERE workspace_id = $2::uuid
		  AND id IN (
		    SELECT activity_id FROM activity_link
		    WHERE person_id = $1::uuid
		  )`,
		personID, wsID,
	); err != nil {
		return fmt.Errorf("eraseNormalized activity: %w", err)
	}

	// 5. Null raw on deals linked via activity chain (person → activity → deal).
	if _, err := tx.ExecContext(
		ctx, `
		UPDATE deal SET raw = NULL
		WHERE workspace_id = $2::uuid
		  AND id IN (
		    SELECT al2.deal_id
		    FROM activity_link al1
		    JOIN activity_link al2 ON al2.activity_id = al1.activity_id
		    WHERE al1.person_id    = $1::uuid
		      AND al2.entity_type  = 'deal'
		      AND al2.deal_id IS NOT NULL
		  )`,
		personID, wsID,
	); err != nil {
		return fmt.Errorf("eraseNormalized deal: %w", err)
	}

	return nil
}

// eraseRawAndVector deletes ALL of the subject's embedding rows from pgvector —
// not only the person vector, but every activity/lead/deal vector that belongs to
// the subject (mirroring the same join paths eraseNormalized uses to null PII).
// gdpr.md promises erasure is *total* across pgvector; a person-only delete would
// leave the subject's text recoverable via the surviving activity/lead/deal
// vectors. The subqueries are PII-free (id-only) so the statement is itself safe.
func eraseRawAndVector(ctx context.Context, tx *sql.Tx, wsID, personID string) error {
	if _, err := tx.ExecContext(
		ctx, `
		DELETE FROM embedding e
		WHERE e.workspace_id = $1::uuid
		  AND (
		    -- the person vector itself
		    (e.source_type = 'person'   AND e.source_id = $2::uuid)
		    -- the source lead the person was converted from
		    OR (e.source_type = 'lead'  AND e.source_id = (
		        SELECT converted_from_lead_id FROM person
		        WHERE id = $2::uuid AND workspace_id = $1::uuid))
		    -- every activity linked to the subject
		    OR (e.source_type = 'activity' AND e.source_id IN (
		        SELECT activity_id FROM activity_link WHERE person_id = $2::uuid))
		    -- every deal reached via the subject's activity chain
		    OR (e.source_type = 'deal'  AND e.source_id IN (
		        SELECT al2.deal_id
		        FROM activity_link al1
		        JOIN activity_link al2 ON al2.activity_id = al1.activity_id
		        WHERE al1.person_id   = $2::uuid
		          AND al2.entity_type = 'deal'
		          AND al2.deal_id IS NOT NULL))
		  )`,
		wsID, personID,
	); err != nil {
		return fmt.Errorf("eraseRawAndVector embedding: %w", err)
	}
	return nil
}

// Erase irreversibly removes PII for a person from normalized tables, deletes
// embeddings, adds emails to the suppression list, and writes a PII-free audit tombstone.
func Erase(ctx context.Context, db *sql.DB, personID string) error {
	if db == nil {
		return fmt.Errorf("crmgdpr.Erase: db is nil")
	}

	wsID := ""
	if p, ok := crmctx.From(ctx); ok {
		wsID = p.TenantID
	}
	if wsID == "" {
		return fmt.Errorf("crmgdpr.Erase: workspace_id is empty")
	}

	return database.WithWorkspaceTx(ctx, db, wsID, func(tx *sql.Tx) error {
		// Capture emails BEFORE eraseNormalized deletes them.
		emails, err := captureEmails(ctx, tx, wsID, personID)
		if err != nil {
			return err
		}

		if err := eraseNormalized(ctx, tx, wsID, personID); err != nil {
			return err
		}

		if err := eraseRawAndVector(ctx, tx, wsID, personID); err != nil {
			return err
		}

		for _, email := range emails {
			if err := suppressionAdd(ctx, tx, wsID, email); err != nil {
				return err
			}
		}

		// Write a PII-free tombstone; actor attribution via ctx.
		entry := buildErasureTombstone(personID)
		entry.WorkspaceID = wsID
		attributeActor(ctx, &entry)

		if _, err := crmaudit.WriteTx(ctx, tx, entry); err != nil {
			return fmt.Errorf("crmgdpr.Erase audit: %w", err)
		}
		return nil
	})
}

// captureEmails reads the subject's emails before they are deleted, so they can be
// added to the suppression list after eraseNormalized runs.
func captureEmails(ctx context.Context, tx *sql.Tx, wsID, personID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT email FROM person_email WHERE person_id=$1::uuid AND workspace_id=$2::uuid`,
		personID, wsID)
	if err != nil {
		return nil, fmt.Errorf("crmgdpr.Erase select emails: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var emails []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("crmgdpr.Erase scan email: %w", err)
		}
		emails = append(emails, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crmgdpr.Erase rows: %w", err)
	}
	return emails, nil
}

// attributeActor fills the actor fields on a GDPR audit entry from the ctx
// principal, falling back to the system actor when no principal is present.
func attributeActor(ctx context.Context, entry *crmaudit.Entry) {
	p, ok := crmctx.From(ctx)
	if !ok || p.UserID == "" {
		entry.ActorType = actorSystem
		entry.ActorID = actorSystem
		return
	}
	entry.ActorID = p.UserID
	if p.IsAgent {
		entry.ActorType = "agent"
		obo := p.UserID
		entry.OnBehalfOf = &obo
		return
	}
	entry.ActorType = "human"
}
