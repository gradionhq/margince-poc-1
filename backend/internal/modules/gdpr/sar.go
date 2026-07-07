package crmgdpr

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// SARPackage holds all data held about a subject for an Art. 15 Subject Access Request.
type SARPackage struct {
	Person        json.RawMessage
	Emails        []json.RawMessage
	Activities    []json.RawMessage
	Deals         []json.RawMessage
	Organizations []json.RawMessage
	RawCapture    []json.RawMessage
}

// sarManifest is stored in audit_log.after — counts only, no PII.
type sarManifest struct {
	Emails        int `json:"emails"`
	Activities    int `json:"activities"`
	Deals         int `json:"deals"`
	Organizations int `json:"organizations"`
	RawCapture    int `json:"raw_capture"`
}

// Assemble gathers all data held about personID and writes a PII-free audit row.
// The ctx Principal must be set (admin actor); the audit row records act + counts, not the payload.
func Assemble(ctx context.Context, db *sql.DB, personID string) (SARPackage, error) {
	if db == nil {
		return SARPackage{}, fmt.Errorf("crmgdpr.Assemble: db is nil")
	}

	wsID := ""
	if p, ok := crmctx.From(ctx); ok {
		wsID = p.TenantID
	}
	if wsID == "" {
		return SARPackage{}, fmt.Errorf("crmgdpr.Assemble: workspace_id is empty")
	}

	var pkg SARPackage
	err := database.WithWorkspaceTx(ctx, db, wsID, func(tx *sql.Tx) error {
		var err error
		pkg, err = gatherSAR(ctx, tx, personID, wsID)
		if err != nil {
			return err
		}

		manifest := sarManifest{
			Emails:        len(pkg.Emails),
			Activities:    len(pkg.Activities),
			Deals:         len(pkg.Deals),
			Organizations: len(pkg.Organizations),
			RawCapture:    len(pkg.RawCapture),
		}
		pid := personID
		entry := crmaudit.Entry{
			WorkspaceID: wsID,
			Action:      "export",
			EntityType:  objectPerson,
			EntityID:    &pid,
			Before:      nil,
			After:       manifest,
		}
		attributeActor(ctx, &entry)

		if _, err := crmaudit.WriteTx(ctx, tx, entry); err != nil {
			return fmt.Errorf("crmgdpr.Assemble audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return SARPackage{}, err
	}
	return pkg, nil
}

// SAR data-gathering queries. $1 = person id, $2 = workspace id. Each returns a
// single json/jsonb column (except sarPersonQuery, which is scanned directly).
const (
	sarPersonQuery = `SELECT row_to_json(p) FROM person p WHERE id=$1::uuid AND workspace_id=$2::uuid`

	sarEmailsQuery = `SELECT row_to_json(e) FROM person_email e WHERE person_id=$1::uuid AND workspace_id=$2::uuid`

	sarActivitiesQuery = `
		SELECT row_to_json(a)
		FROM activity a
		WHERE a.workspace_id=$2::uuid
		  AND a.id IN (
		    SELECT activity_id FROM activity_link
		    WHERE person_id=$1::uuid AND workspace_id=$2::uuid
		  )`

	sarDealsQuery = `
		SELECT row_to_json(d)
		FROM deal d
		WHERE d.workspace_id=$2::uuid
		  AND d.id IN (
		    SELECT al2.deal_id
		    FROM activity_link al1
		    JOIN activity_link al2 ON al2.activity_id = al1.activity_id
		    WHERE al1.person_id    = $1::uuid
		      AND al2.entity_type  = 'deal'
		      AND al2.deal_id IS NOT NULL
		  )`

	sarOrgsQuery = `
		SELECT row_to_json(o)
		FROM (
		  SELECT DISTINCT ON (o.id) o.*
		  FROM organization o
		  WHERE o.workspace_id=$2::uuid
		    AND o.id IN (
		      SELECT d.organization_id
		      FROM deal d
		      WHERE d.workspace_id=$2::uuid
		        AND d.organization_id IS NOT NULL
		        AND d.id IN (
		          SELECT al2.deal_id
		          FROM activity_link al1
		          JOIN activity_link al2 ON al2.activity_id = al1.activity_id
		          WHERE al1.person_id   = $1::uuid
		            AND al2.entity_type = 'deal'
		            AND al2.deal_id IS NOT NULL
		        )
		    )
		  ORDER BY o.id
		) o`

	sarRawQuery = `
		SELECT raw FROM (
		  SELECT raw FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid AND raw IS NOT NULL
		  UNION ALL
		  SELECT a.raw FROM activity a
		  WHERE a.workspace_id=$2::uuid AND a.raw IS NOT NULL
		    AND a.id IN (SELECT activity_id FROM activity_link WHERE person_id=$1::uuid AND workspace_id=$2::uuid)
		  UNION ALL
		  SELECT d.raw FROM deal d
		  WHERE d.workspace_id=$2::uuid AND d.raw IS NOT NULL
		    AND d.id IN (
		      SELECT al2.deal_id
		      FROM activity_link al1
		      JOIN activity_link al2 ON al2.activity_id=al1.activity_id
		      WHERE al1.person_id=$1::uuid AND al2.entity_type='deal' AND al2.deal_id IS NOT NULL
		    )
		) raws`
)

// gatherSAR reads every category of data held about the subject within tx
// (already workspace-scoped) into a SARPackage.
func gatherSAR(ctx context.Context, tx *sql.Tx, personID, wsID string) (SARPackage, error) {
	var pkg SARPackage

	var personJSON []byte
	if err := tx.QueryRowContext(ctx, sarPersonQuery, personID, wsID).Scan(&personJSON); err != nil {
		return SARPackage{}, fmt.Errorf("crmgdpr.Assemble person: %w", err)
	}
	pkg.Person = personJSON

	var err error
	if pkg.Emails, err = queryJSONRows(ctx, tx, "emails", sarEmailsQuery, personID, wsID); err != nil {
		return SARPackage{}, err
	}
	if pkg.Activities, err = queryJSONRows(ctx, tx, "activities", sarActivitiesQuery, personID, wsID); err != nil {
		return SARPackage{}, err
	}
	if pkg.Deals, err = queryJSONRows(ctx, tx, "deals", sarDealsQuery, personID, wsID); err != nil {
		return SARPackage{}, err
	}
	if pkg.Organizations, err = queryJSONRows(ctx, tx, "organizations", sarOrgsQuery, personID, wsID); err != nil {
		return SARPackage{}, err
	}
	if pkg.RawCapture, err = queryJSONRows(ctx, tx, "raw", sarRawQuery, personID, wsID); err != nil {
		return SARPackage{}, err
	}

	return pkg, nil
}

// queryJSONRows runs a single-column json/jsonb query and collects its rows,
// wrapping any failure with the section label for an Assemble error.
func queryJSONRows(ctx context.Context, tx *sql.Tx, label, query string, args ...any) ([]json.RawMessage, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("crmgdpr.Assemble %s: %w", label, err)
	}
	out, err := scanJSONRows(rows)
	if err != nil {
		return nil, fmt.Errorf("crmgdpr.Assemble %s scan: %w", label, err)
	}
	return out, nil
}

// scanJSONRows reads all rows from a single-column json/jsonb query into a slice.
func scanJSONRows(rows *sql.Rows) ([]json.RawMessage, error) {
	defer func() { _ = rows.Close() }()
	var out []json.RawMessage
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(b))
	}
	return out, rows.Err()
}
