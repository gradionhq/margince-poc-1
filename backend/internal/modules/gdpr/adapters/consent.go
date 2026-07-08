package adapters

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/gdpr/domain"
	"github.com/gradionhq/margince/backend/internal/modules/gdpr/ports"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
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
		WHERE pc.person_id = $1::uuid AND cp.key = $2 AND pc.workspace_id = $3::uuid`,
		personID, purposeName, workspaceID).Scan(&state)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("crmgdpr.Check: %w", err)
	}
	return state == string(domain.Granted), nil
}

// ConsentRequest carries everything needed to record one consent signal.
type ConsentRequest struct {
	WorkspaceID    string
	PersonID       string
	PurposeName    string
	NewState       domain.ConsentState // must be Granted or Withdrawn
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
	if w.NewState != domain.Granted && w.NewState != domain.Withdrawn {
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
		`SELECT id FROM consent_purpose WHERE workspace_id = $1 AND key = $2`, wsID, w.PurposeName,
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

// NewConsentRepository returns a PostgreSQL-backed ports.ConsentRepository.
//
//nolint:ireturn // seam returns the ConsentRepository interface by design
func NewConsentRepository(db *sql.DB) ports.ConsentRepository {
	return &pgConsentRepository{db: db}
}

type pgConsentRepository struct{ db *sql.DB }

// FindForPurpose returns Granted/Withdrawn/Unknown. Default-deny: no row or wrong
// purpose → Unknown.
func (r *pgConsentRepository) FindForPurpose(ctx context.Context, workspaceID, personID, purpose string) (domain.ConsentState, error) {
	if r.db == nil {
		return domain.Unknown, fmt.Errorf("crmgdpr.FindForPurpose: db is nil")
	}
	var state string
	err := database.WithWorkspaceTx(ctx, r.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(
			ctx, `
			SELECT pc.state
			FROM person_consent pc
			JOIN consent_purpose cp ON cp.id = pc.purpose_id
			WHERE pc.person_id    = $1::uuid
			  AND cp.key          = $2
			  AND pc.workspace_id = $3::uuid`,
			personID, purpose, workspaceID,
		).Scan(&state)
	})
	if err == sql.ErrNoRows {
		return domain.Unknown, nil
	}
	if err != nil {
		return domain.Unknown, fmt.Errorf("crmgdpr.FindForPurpose: %w", err)
	}
	switch domain.ConsentState(state) {
	case domain.Granted:
		return domain.Granted, nil
	case domain.Withdrawn:
		return domain.Withdrawn, nil
	default:
		return domain.Unknown, nil
	}
}

// nullableStr returns nil for an empty string (maps to SQL NULL).
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
