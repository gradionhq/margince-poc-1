// Package adapters — OrgStore CRUD (organizations module, WS-E-a).
// Ported from modules/directory/store_org.go (package crmcore → package adapters).
// withWorkspaceTx → database.WithWorkspaceTx.
package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/dedupe"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

// OrgStore executes parameterized SQL against the organization table.
type OrgStore struct {
	db          *sql.DB
	personStore *PersonStore
}

// NewOrgStore returns an OrgStore.
func NewOrgStore(db *sql.DB) *OrgStore {
	return &OrgStore{db: db, personStore: NewPersonStore(db)}
}

// ErrDuplicateDomain reports a normalized-domain collision during Create.
type ErrDuplicateDomain struct {
	ExistingID string
	Field      string
}

func (e *ErrDuplicateDomain) Error() string {
	return fmt.Sprintf("duplicate domain: existing_id=%s field=%s", e.ExistingID, e.Field)
}

// Create inserts an organization and its domains in one workspace-scoped tx.
func (s *OrgStore) Create(ctx context.Context, o domain.Organization) (domain.Organization, error) {
	if err := sqlutil.RequireProvenance(o.Source, o.CapturedBy); err != nil {
		return domain.Organization{}, err
	}
	o.ID = ids.New()
	social := sqlutil.MarshalJSON(o.Social)
	address := sqlutil.MarshalJSON(o.Address)
	classification := o.Classification
	if classification == nil {
		def := "prospect"
		classification = &def
	}
	domains := make([]domain.OrganizationDomain, len(o.Domains))
	copy(domains, o.Domains)
	hasPrimary := false
	for i := range domains {
		domains[i].Domain = strings.ToLower(strings.TrimSpace(domains[i].Domain))
		if domains[i].IsPrimary {
			hasPrimary = true
		}
	}
	if len(domains) > 0 && !hasPrimary {
		domains[0].IsPrimary = true
	}
	o.Classification = classification
	o.Domains = domains
	var reviewFlag *dedupe.ReviewFlag
	err := database.WithWorkspaceTx(ctx, s.db, o.WorkspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO organization (id, workspace_id, name, website, classification, relevance,
			    owner_id, social, address, source, captured_by, version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,1)`,
			o.ID, o.WorkspaceID, o.DisplayName, o.Website, classification, o.Relevance,
			o.OwnerID, social, address,
			o.Source, o.CapturedBy)
		if err != nil {
			return err
		}
		if err := insertOrgDomains(ctx, tx, o.WorkspaceID, o.ID, domains); err != nil {
			return err
		}
		// PO-AC-19: the name-only fuzzy tier only runs once the exact-domain
		// tier has already succeeded (no 409) — a non-blocking review-flag.
		flag, err := s.fuzzyDedupe(ctx, tx, o.WorkspaceID, o.ID, o.DisplayName)
		if err != nil {
			return err
		}
		reviewFlag = flag
		payload, _ := json.Marshal(map[string]any{fieldOrganizationID: o.ID})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload)
			 VALUES ($1,$2,$3::uuid,$4)`,
			o.WorkspaceID, "organization.created", o.ID, payload); err != nil {
			return fmt.Errorf("org create event: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeOrganization, &o.ID, nil, o)
		e.WorkspaceID = o.WorkspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("org create audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Organization{}, err
	}
	created, err := s.Get(ctx, o.ID, o.WorkspaceID)
	if err != nil {
		return domain.Organization{}, err
	}
	created.ReviewFlag = reviewFlag
	return created, nil
}

func insertOrgDomains(ctx context.Context, tx *sql.Tx, workspaceID, orgID string, domains []domain.OrganizationDomain) error {
	for i, d := range domains {
		var existingID string
		scanErr := tx.QueryRowContext(ctx, `
			SELECT organization_id FROM organization_domain
			WHERE workspace_id=$1::uuid AND domain=$2 AND archived_at IS NULL`,
			workspaceID, d.Domain).Scan(&existingID)
		if scanErr == nil {
			return &ErrDuplicateDomain{
				ExistingID: existingID,
				Field:      fmt.Sprintf("domains[%d].domain", i),
			}
		}
		if !errors.Is(scanErr, sql.ErrNoRows) {
			return scanErr
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO organization_domain (workspace_id, organization_id, domain, is_primary)
			VALUES ($1,$2,$3,$4)`,
			workspaceID, orgID, d.Domain, d.IsPrimary); err != nil {
			return fmt.Errorf("org create domain: %w", err)
		}
	}
	return nil
}

// Get returns a live organization by id, workspace-scoped; ErrNotFound if absent.
//
//nolint:dupl // parallel per-entity CRUD: the SQL column list and Scan targets differ by type; a generic extraction would read worse than the explicit form
func (s *OrgStore) Get(ctx context.Context, id, workspaceID string) (domain.Organization, error) {
	var o domain.Organization
	var socialRaw, addrRaw []byte
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, name, website, classification, relevance,
			       owner_id, social, address, parent_org_id, merged_into_id,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM organization WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID).Scan(
			&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Website, &o.Classification, &o.Relevance,
			&o.OwnerID, &socialRaw, &addrRaw, &o.ParentOrgID, &o.MergedIntoID,
			&o.Version, &o.Source, &o.CapturedBy,
			&o.CreatedAt, &o.UpdatedAt, &o.ArchivedAt,
		)
		if err != nil {
			return err
		}
		if err := attachOrgDomains(ctx, tx, workspaceID, &o); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, sql.ErrNoRows) {
		return o, errs.ErrNotFound
	}
	if err != nil {
		return o, err
	}
	o.Social = map[string]any{}
	sqlutil.UnmarshalJSON(socialRaw, &o.Social)
	if addrRaw != nil {
		o.Address = map[string]any{}
		sqlutil.UnmarshalJSON(addrRaw, &o.Address)
	}
	return o, nil
}

// Update applies partial updates to an organization, writes one audit_log row and one
// organization.updated outbox event in the same tx (PO-AC-3, GATE-CORE-3/5); ifMatch==0
// skips the version check (last-write-wins).
func (s *OrgStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Organization, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var res sql.Result
		var err error
		if ifMatch == 0 {
			res, err = tx.ExecContext(ctx, `
				UPDATE organization
				SET name       = COALESCE($3, name),
				    website    = COALESCE($4, website),
				    owner_id   = COALESCE($5, owner_id),
				    updated_at = now()
				WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
				id, workspaceID,
				sqlutil.NullStr(updates, "display_name"),
				sqlutil.NullStr(updates, "website"),
				sqlutil.NullStr(updates, "owner_id"))
		} else {
			res, err = tx.ExecContext(ctx, `
				UPDATE organization
				SET name       = COALESCE($3, name),
				    website    = COALESCE($4, website),
				    owner_id   = COALESCE($5, owner_id),
				    updated_at = now()
				WHERE id=$1::uuid AND workspace_id=$2::uuid AND version=$6 AND archived_at IS NULL`,
				id, workspaceID,
				sqlutil.NullStr(updates, "display_name"),
				sqlutil.NullStr(updates, "website"),
				sqlutil.NullStr(updates, "owner_id"),
				ifMatch)
		}
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if ifMatch != 0 {
				return errs.ErrVersionSkew
			}
			return errs.ErrNotFound
		}
		payload, _ := json.Marshal(map[string]any{fieldOrganizationID: id})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "organization.updated", id, payload); err != nil {
			return fmt.Errorf("org update event: %w", err)
		}
		eu := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeOrganization, &id, nil, nil)
		eu.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, eu); err != nil {
			return fmt.Errorf("org update audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Organization{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes an organization, writing one audit_log row and one
// organization.archived outbox event in the same tx when a row was actually
// archived (mirrors PersonStore.Archive's n>0 guard).
func (s *OrgStore) Archive(ctx context.Context, id, workspaceID string) (domain.Organization, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE organization SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			payload, _ := json.Marshal(map[string]any{fieldOrganizationID: id})
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
				workspaceID, "organization.archived", id, payload); err != nil {
				return fmt.Errorf("org archive event: %w", err)
			}
			ea := crmaudit.EntryFromPrincipal(ctx, "archive", entityTypeOrganization, &id, nil, nil)
			ea.WorkspaceID = workspaceID
			if _, err := crmaudit.WriteTx(ctx, tx, ea); err != nil {
				return fmt.Errorf("org archive audit: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.Organization{}, err
	}
	return s.GetAny(ctx, id, workspaceID)
}

// Restore clears archived_at, restoring an organization to default list visibility.
// It refuses live records, merged records, and restores that would collide with an
// active domain on another organization.
func (s *OrgStore) Restore(ctx context.Context, id, workspaceID string) (domain.Organization, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var archivedAt sql.NullTime
		var mergedInto sql.NullString
		if err := tx.QueryRowContext(ctx, `
			SELECT archived_at, merged_into_id
			FROM organization
			WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(&archivedAt, &mergedInto); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errs.ErrNotFound
			}
			return err
		}
		if !archivedAt.Valid {
			return errs.ErrNotArchived
		}
		if mergedInto.Valid {
			return errs.ErrMergedRecord
		}

		var existingID string
		err := tx.QueryRowContext(ctx, `
			SELECT od2.organization_id
			FROM organization_domain od1
			JOIN organization_domain od2 ON od2.domain = od1.domain
			  AND od2.workspace_id = od1.workspace_id
			  AND od2.organization_id <> od1.organization_id
			  AND od2.archived_at IS NULL
			WHERE od1.organization_id=$1::uuid AND od1.workspace_id=$2::uuid AND od1.archived_at IS NULL
			LIMIT 1`,
			id, workspaceID).Scan(&existingID)
		if err == nil {
			return &ErrDuplicateDomain{ExistingID: existingID, Field: "domain"}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE organization SET archived_at=NULL WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NOT NULL`,
			id, workspaceID); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{fieldOrganizationID: id})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "organization.restored", id, payload); err != nil {
			return fmt.Errorf("org restore event: %w", err)
		}
		er := crmaudit.EntryFromPrincipal(ctx, "restore", entityTypeOrganization, &id, nil, nil)
		er.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, er); err != nil {
			return fmt.Errorf("org restore audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Organization{}, err
	}
	return s.GetAny(ctx, id, workspaceID)
}

// GetAny fetches an organization by id regardless of archived_at status.
//
//nolint:dupl // parallel per-entity CRUD: the SQL column list and Scan targets differ by type; a generic extraction would read worse than the explicit form
func (s *OrgStore) GetAny(ctx context.Context, id, workspaceID string) (domain.Organization, error) {
	var o domain.Organization
	var socialRaw, addrRaw []byte
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, name, website, classification, relevance,
			       owner_id, social, address, parent_org_id, merged_into_id,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM organization WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(
			&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Website, &o.Classification, &o.Relevance,
			&o.OwnerID, &socialRaw, &addrRaw, &o.ParentOrgID, &o.MergedIntoID,
			&o.Version, &o.Source, &o.CapturedBy,
			&o.CreatedAt, &o.UpdatedAt, &o.ArchivedAt,
		)
		if err != nil {
			return err
		}
		if err := attachOrgDomains(ctx, tx, workspaceID, &o); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, sql.ErrNoRows) {
		return o, errs.ErrNotFound
	}
	if err != nil {
		return o, err
	}
	o.Social = map[string]any{}
	sqlutil.UnmarshalJSON(socialRaw, &o.Social)
	if addrRaw != nil {
		o.Address = map[string]any{}
		sqlutil.UnmarshalJSON(addrRaw, &o.Address)
	}
	return o, nil
}

func attachOrgDomains(ctx context.Context, tx *sql.Tx, workspaceID string, o *domain.Organization) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, organization_id, domain, is_primary, created_at, updated_at, archived_at
		FROM organization_domain
		WHERE workspace_id=$1::uuid AND organization_id=$2::uuid AND archived_at IS NULL
		ORDER BY is_primary DESC, domain`,
		workspaceID, o.ID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	domains := []domain.OrganizationDomain{}
	for rows.Next() {
		var d domain.OrganizationDomain
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Domain, &d.IsPrimary,
			&d.CreatedAt, &d.UpdatedAt, &d.ArchivedAt); err != nil {
			return err
		}
		domains = append(domains, d)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	o.Domains = domains
	return nil
}
