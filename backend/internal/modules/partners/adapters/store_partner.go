// Package adapters contains the partners module's PostgreSQL storage adapters.
package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/partners/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

const (
	entityTypePartner   = "partner"
	fieldOrganizationID = "organization_id"
)

// PartnerStore executes parameterized SQL against the partner table.
type PartnerStore struct{ db *sql.DB }

// NewPartnerStore returns a PartnerStore backed by db.
func NewPartnerStore(db *sql.DB) *PartnerStore { return &PartnerStore{db: db} }

type partnerRowScanner interface {
	Scan(dest ...any) error
}

func scanPartnerRow(row partnerRowScanner) (domain.Partner, error) {
	var p domain.Partner
	var joinedAt, renewsAt, archivedAt sql.NullTime
	var retentionRate sql.NullFloat64
	if err := row.Scan(
		&p.ID, &p.WorkspaceID, &p.OrganizationID, &p.CertStatus, &p.PartnerRole, &p.MarginTier,
		&p.CertifiedStaff, &retentionRate, &joinedAt, &renewsAt,
		&p.Version, &p.Source, &p.CapturedBy, &p.CreatedAt, &p.UpdatedAt, &archivedAt,
	); err != nil {
		return domain.Partner{}, err
	}
	p.GateMetrics = map[string]any{}
	if retentionRate.Valid {
		r := retentionRate.Float64
		p.RetentionRate = &r
	}
	if joinedAt.Valid {
		t := joinedAt.Time
		p.JoinedAt = &t
	}
	if renewsAt.Valid {
		t := renewsAt.Time
		p.RenewsAt = &t
	}
	if archivedAt.Valid {
		t := archivedAt.Time
		p.ArchivedAt = &t
	}
	return p, nil
}

// Upsert creates or updates the 1:1 partner row for p.OrganizationID and
// keeps the owning organization classified as partner in the same transaction.
func (s *PartnerStore) Upsert(ctx context.Context, p domain.Partner) (domain.Partner, error) {
	if err := requireProvenance(p.Source, p.CapturedBy); err != nil {
		return domain.Partner{}, err
	}
	if p.ID == "" {
		p.ID = ids.New()
	}

	var partnerID string
	var inserted bool
	err := database.WithWorkspaceTx(ctx, s.db, p.WorkspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO partner (
			    id, workspace_id, organization_id, cert_status, partner_role, margin_tier,
			    certified_staff, retention_rate, joined_at, renews_at, source, captured_by, version
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,1)
			ON CONFLICT (organization_id) DO UPDATE SET
			    cert_status     = EXCLUDED.cert_status,
			    partner_role    = EXCLUDED.partner_role,
			    margin_tier     = EXCLUDED.margin_tier,
			    certified_staff = EXCLUDED.certified_staff,
			    retention_rate  = EXCLUDED.retention_rate,
			    joined_at       = EXCLUDED.joined_at,
			    renews_at       = EXCLUDED.renews_at,
			    updated_at      = now()
			RETURNING id, (xmax = 0) AS inserted`,
			p.ID, p.WorkspaceID, p.OrganizationID, p.CertStatus, p.PartnerRole, p.MarginTier,
			p.CertifiedStaff, p.RetentionRate, p.JoinedAt, p.RenewsAt,
			p.Source, p.CapturedBy)
		if err := row.Scan(&partnerID, &inserted); err != nil {
			return fmt.Errorf("partner upsert: %w", err)
		}

		res, err := tx.ExecContext(ctx,
			`UPDATE organization SET classification='partner' WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			p.OrganizationID, p.WorkspaceID)
		if err != nil {
			return fmt.Errorf("partner upsert org classification: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errs.ErrNotFound
		}

		action, topic := "update", "partner.updated"
		if inserted {
			action, topic = "create", "partner.created"
		}
		payload, _ := json.Marshal(map[string]any{fieldOrganizationID: p.OrganizationID})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			p.WorkspaceID, topic, partnerID, payload); err != nil {
			return fmt.Errorf("partner upsert event: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, action, entityTypePartner, &partnerID, nil, p)
		e.WorkspaceID = p.WorkspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("partner upsert audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Partner{}, err
	}
	return s.Get(ctx, p.OrganizationID, p.WorkspaceID)
}

// Get returns the live partner row extending organizationID; ErrNotFound if absent.
func (s *PartnerStore) Get(ctx context.Context, organizationID, workspaceID string) (domain.Partner, error) {
	var p domain.Partner
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, organization_id, cert_status, partner_role, margin_tier,
			       certified_staff, retention_rate, joined_at, renews_at,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM partner
			WHERE organization_id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			organizationID, workspaceID)
		var scanErr error
		p, scanErr = scanPartnerRow(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return p, errs.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	return p, nil
}

// List returns a page of live partner rows, filtered by partner_role and cert_status.
func (s *PartnerStore) List(ctx context.Context, workspaceID, cursor string, limit int, filter domain.PartnerListFilter) ([]domain.Partner, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	out := []domain.Partner{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		args := []any{workspaceID, cursor, limit + 1}
		where := ""
		if filter.PartnerRole != "" {
			args = append(args, filter.PartnerRole)
			where += fmt.Sprintf(" AND partner_role=$%d", len(args))
		}
		if filter.CertStatus != "" {
			args = append(args, filter.CertStatus)
			where += fmt.Sprintf(" AND cert_status=$%d", len(args))
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, organization_id, cert_status, partner_role, margin_tier,
			       certified_staff, retention_rate, joined_at, renews_at,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM partner
			WHERE workspace_id=$1::uuid AND archived_at IS NULL
			  AND ($2 = '' OR id::text > $2)`+where+`
			ORDER BY id LIMIT $3`,
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			p, scanErr := scanPartnerRow(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", err
	}

	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}
