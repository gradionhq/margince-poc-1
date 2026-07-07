// Package crmgdpr implements GDPR engine: consent, retention, erasure and SAR.
package crmgdpr

import (
	"context"
	"database/sql"
	"fmt"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// ConsentState is the current consent status for a given (person, purpose) pair.
type ConsentState string

// The consent states a (person, purpose) pair can be in.
const (
	Granted   ConsentState = "granted"
	Withdrawn ConsentState = "withdrawn"
	Unknown   ConsentState = "unknown"
)

// consentQueryer is the read surface Check needs — satisfied by *sql.Tx and *sql.DB.
type consentQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Check reports whether personID has granted consent for purposeName in workspaceID.
// q must already be workspace-scoped (the caller runs it inside a withWorkspaceTx, so
// the margince_app role + app.workspace_id are set) — Check does NOT call set_config,
// avoiding the pooled-connection GUC pollution the earlier standalone form had.
// Default-deny: no row ⇒ false.
func Check(ctx context.Context, q consentQueryer, workspaceID, personID, purposeName string) (bool, error) {
	var state string
	err := q.QueryRowContext(ctx, `
		SELECT pc.state
		FROM person_consent pc
		JOIN consent_purpose cp ON cp.id = pc.purpose_id
		WHERE pc.person_id = $1::uuid AND cp.name = $2 AND pc.workspace_id = $3::uuid`,
		personID, purposeName, workspaceID).Scan(&state)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("crmgdpr.Check: %w", err)
	}
	return state == string(Granted), nil
}

// ConsentRequest carries everything needed to record one consent signal.
type ConsentRequest struct {
	WorkspaceID    string
	PersonID       string
	PurposeName    string
	NewState       ConsentState // must be Granted or Withdrawn
	Channel        string
	LawfulBasis    string
	PolicyWording  string
	PolicyVersion  string
	DoubleOptInRef string
	Source         string
}

// Record records a consent signal in one atomic transaction:
//  1. Resolves purpose_id by name.
//  2. Upserts person_consent (sets state).
//  3. Inserts one consent_event proof row.
//  4. Writes one audit_log row via crmaudit.WriteTx (Action="update", EntityType="person_consent").
func Record(ctx context.Context, db *sql.DB, w ConsentRequest) error {
	if db == nil {
		return fmt.Errorf("crmgdpr.Record: db is nil")
	}
	if w.NewState != Granted && w.NewState != Withdrawn {
		return fmt.Errorf("crmgdpr.Record: NewState must be granted or withdrawn, got %q", w.NewState)
	}

	wsID := w.WorkspaceID
	if wsID == "" {
		if p, ok := crmctx.From(ctx); ok {
			wsID = p.TenantID
		}
	}
	if wsID == "" {
		return fmt.Errorf("crmgdpr.Record: workspace_id is empty")
	}

	return database.WithWorkspaceTx(ctx, db, wsID, func(tx *sql.Tx) error {
		// 1-3. Persist the consent state + proof row, returning the consent id.
		consentID, err := persistConsent(ctx, tx, wsID, w)
		if err != nil {
			return err
		}

		// 4. Audit the consent change.
		authRule := "gdpr.consent"
		entry := crmaudit.EntryFromPrincipal(ctx, "update", "person_consent", &consentID, nil, nil)
		entry.WorkspaceID = wsID
		entry.AuthorizationRule = &authRule
		if _, err := crmaudit.WriteTx(ctx, tx, entry); err != nil {
			return fmt.Errorf("crmgdpr.Record audit: %w", err)
		}
		return nil
	})
}

// persistConsent resolves the purpose, upserts person_consent, and writes the
// consent_event proof row, returning the upserted person_consent id.
func persistConsent(ctx context.Context, tx *sql.Tx, wsID string, w ConsentRequest) (string, error) {
	var purposeID string
	if err := tx.QueryRowContext(
		ctx,
		`SELECT id FROM consent_purpose WHERE name = $1`, w.PurposeName,
	).Scan(&purposeID); err != nil {
		return "", fmt.Errorf("crmgdpr.Record resolve purpose %q: %w", w.PurposeName, err)
	}

	var consentID string
	if err := tx.QueryRowContext(
		ctx, `
		INSERT INTO person_consent
		  (workspace_id, person_id, purpose_id, state, lawful_basis, captured_at, source, policy_version)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, now(), $6, $7)
		ON CONFLICT (workspace_id, person_id, purpose_id)
		DO UPDATE SET
		  state          = EXCLUDED.state,
		  lawful_basis   = EXCLUDED.lawful_basis,
		  captured_at    = now(),
		  source         = EXCLUDED.source,
		  policy_version = EXCLUDED.policy_version
		RETURNING id`,
		wsID, w.PersonID, purposeID, string(w.NewState),
		nullableStr(w.LawfulBasis), w.Source, nullableStr(w.PolicyVersion),
	).Scan(&consentID); err != nil {
		return "", fmt.Errorf("crmgdpr.Record upsert person_consent: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx, `
		INSERT INTO consent_event
		  (workspace_id, person_id, purpose_id, event_state, channel, lawful_basis,
		   policy_wording, policy_version, double_opt_in_ref, source)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10)`,
		wsID, w.PersonID, purposeID, string(w.NewState),
		nullableStr(w.Channel), nullableStr(w.LawfulBasis),
		w.PolicyWording, w.PolicyVersion,
		nullableStr(w.DoubleOptInRef), w.Source,
	); err != nil {
		return "", fmt.Errorf("crmgdpr.Record insert consent_event: %w", err)
	}

	return consentID, nil
}

// ConsentRepository is the GDPR consent read seam for per-call consent checks.
// workspaceID is an explicit param (not derived from ctx) so callers without a
// crmctx principal can still query — matching the seam doc (N3).
type ConsentRepository interface {
	FindForPurpose(ctx context.Context, workspaceID, personID, purpose string) (ConsentState, error)
}

// NewConsentRepository returns a PostgreSQL-backed ConsentRepository.
//
//nolint:ireturn // seam returns the ConsentRepository interface by design
func NewConsentRepository(db *sql.DB) ConsentRepository {
	return &pgConsentRepository{db: db}
}

type pgConsentRepository struct{ db *sql.DB }

// FindForPurpose returns Granted/Withdrawn/Unknown. Default-deny: no row or wrong
// purpose → Unknown. Caller is responsible for setting the RLS GUC if needed.
func (r *pgConsentRepository) FindForPurpose(ctx context.Context, workspaceID, personID, purpose string) (ConsentState, error) {
	if r.db == nil {
		return Unknown, fmt.Errorf("crmgdpr.FindForPurpose: db is nil")
	}
	var state string
	err := r.db.QueryRowContext(
		ctx, `
		SELECT pc.state
		FROM person_consent pc
		JOIN consent_purpose cp ON cp.id = pc.purpose_id
		WHERE pc.person_id    = $1::uuid
		  AND cp.name         = $2
		  AND pc.workspace_id = $3::uuid`,
		personID, purpose, workspaceID,
	).Scan(&state)
	if err == sql.ErrNoRows {
		return Unknown, nil
	}
	if err != nil {
		return Unknown, fmt.Errorf("crmgdpr.FindForPurpose: %w", err)
	}
	switch ConsentState(state) {
	case Granted:
		return Granted, nil
	case Withdrawn:
		return Withdrawn, nil
	default:
		return Unknown, nil
	}
}

// nullableStr returns nil for an empty string (maps to SQL NULL).
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
