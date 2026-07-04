package crmcore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// ---------------------------------------------------------------------------
// OrgStore
// ---------------------------------------------------------------------------

// OrgStore executes parameterized SQL against the organization table.
type OrgStore struct {
	db          *sql.DB
	personStore *PersonStore
}

// NewOrgStore returns an OrgStore.
func NewOrgStore(db *sql.DB) *OrgStore {
	return &OrgStore{db: db, personStore: NewPersonStore(db)}
}

// ErrDuplicateDomain is returned by OrgStore.Create when a normalized domain in
// the request already maps to another live (archived_at IS NULL)
// organization_domain row in the same workspace. Mirrors ErrLeadEmailDuplicate
// (store_lead.go) - a request-scoped collision, not a fixed Tier-0 sentinel.
type ErrDuplicateDomain struct {
	ExistingID string
	Field      string
}

func (e *ErrDuplicateDomain) Error() string {
	return fmt.Sprintf("duplicate domain: existing_id=%s field=%s", e.ExistingID, e.Field)
}

// Create inserts an organization and its domains in one workspace-scoped tx.
func (s *OrgStore) Create(ctx context.Context, o Organization) (Organization, error) {
	if err := requireProvenance(o.Source, o.CapturedBy); err != nil {
		return Organization{}, err
	}
	o.ID = ids.New()
	social := marshalJSON(o.Social)
	address := marshalJSON(o.Address)

	classification := o.Classification
	if classification == nil {
		def := "prospect"
		classification = &def
	}

	domains := make([]OrganizationDomain, len(o.Domains))
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

	err := withWorkspaceTx(ctx, s.db, o.WorkspaceID, func(tx *sql.Tx) error {
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

		for i, d := range domains {
			var existingID string
			scanErr := tx.QueryRowContext(ctx, `
				SELECT organization_id FROM organization_domain
				WHERE workspace_id=$1::uuid AND domain=$2 AND archived_at IS NULL`,
				o.WorkspaceID, d.Domain).Scan(&existingID)
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
				o.WorkspaceID, o.ID, d.Domain, d.IsPrimary); err != nil {
				return fmt.Errorf("org create domain: %w", err)
			}
		}

		payload, _ := json.Marshal(map[string]any{"organization_id": o.ID})
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
		return Organization{}, err
	}
	return s.Get(ctx, o.ID, o.WorkspaceID)
}

// Get returns a live organization by id, workspace-scoped; ErrNotFound if absent.
//
//nolint:dupl // parallel per-entity CRUD: the SQL column list and Scan targets differ by type; a generic extraction would read worse than the explicit form
func (s *OrgStore) Get(ctx context.Context, id, workspaceID string) (Organization, error) {
	var o Organization
	var socialRaw, addrRaw []byte
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
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
	unmarshalJSON(socialRaw, &o.Social)
	if addrRaw != nil {
		o.Address = map[string]any{}
		unmarshalJSON(addrRaw, &o.Address)
	}
	return o, nil
}

// List returns a page of live organizations. sort="" or "id" uses ID keyset cursor;
// "strength"/"-strength" fetches all, attaches aggregates, sorts by score, offset-paginates.
func (s *OrgStore) List(ctx context.Context, workspaceID, cursor string, limit int, sortVal string) ([]Organization, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	switch sortVal {
	case "strength":
		return s.listByOrgStrength(ctx, workspaceID, cursor, limit, false)
	case "-strength":
		return s.listByOrgStrength(ctx, workspaceID, cursor, limit, true)
	default:
		return s.listByOrgID(ctx, workspaceID, cursor, limit)
	}
}

func (s *OrgStore) listByOrgID(ctx context.Context, workspaceID, cursor string, limit int) ([]Organization, string, error) {
	out := []Organization{}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, name, website, classification, relevance,
			       owner_id, social, version, source, captured_by, created_at, updated_at
			FROM organization
			WHERE workspace_id=$1::uuid AND archived_at IS NULL
			  AND ($2 = '' OR id::text > $2)
			ORDER BY id LIMIT $3`,
			workspaceID, cursor, limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var o Organization
			var socialRaw []byte
			if err := rows.Scan(&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Website, &o.Classification, &o.Relevance,
				&o.OwnerID, &socialRaw, &o.Version, &o.Source, &o.CapturedBy,
				&o.CreatedAt, &o.UpdatedAt); err != nil {
				return err
			}
			o.Social = map[string]any{}
			unmarshalJSON(socialRaw, &o.Social)
			out = append(out, o)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*Organization, len(out))
		for i := range out {
			ptrs[i] = &out[i]
		}
		return attachOrgAggregates(ctx, tx, s.personStore.strengthActivitiesFor, workspaceID, ptrs)
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

func (s *OrgStore) listByOrgStrength(ctx context.Context, workspaceID, cursor string, limit int, descending bool) ([]Organization, string, error) {
	offset := decodeOffsetCursor(cursor)
	all := []Organization{}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, name, website, classification, relevance,
			       owner_id, social, version, source, captured_by, created_at, updated_at
			FROM organization
			WHERE workspace_id=$1::uuid AND archived_at IS NULL
			ORDER BY id`,
			workspaceID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var o Organization
			var socialRaw []byte
			if err := rows.Scan(&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Website, &o.Classification, &o.Relevance,
				&o.OwnerID, &socialRaw, &o.Version, &o.Source, &o.CapturedBy,
				&o.CreatedAt, &o.UpdatedAt); err != nil {
				return err
			}
			o.Social = map[string]any{}
			unmarshalJSON(socialRaw, &o.Social)
			all = append(all, o)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*Organization, len(all))
		for i := range all {
			ptrs[i] = &all[i]
		}
		return attachOrgAggregates(ctx, tx, s.personStore.strengthActivitiesFor, workspaceID, ptrs)
	})
	if err != nil {
		return nil, "", err
	}
	sort.SliceStable(all, func(i, j int) bool {
		si, sj := all[i].Strength, all[j].Strength
		if si == nil && sj == nil {
			return all[i].ID < all[j].ID
		}
		if si == nil {
			return false
		}
		if sj == nil {
			return true
		}
		if si.Score != sj.Score {
			if descending {
				return si.Score > sj.Score
			}
			return si.Score < sj.Score
		}
		return all[i].ID < all[j].ID
	})
	if offset > len(all) {
		offset = len(all)
	}
	end := offset + limit
	var next string
	if end < len(all) {
		next = encodeOffsetCursor(end)
	} else {
		end = len(all)
	}
	return all[offset:end], next, nil
}

// Update applies partial updates to an organization.
// When ifMatch==0 the version check is skipped (last-write-wins).
func (s *OrgStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (Organization, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
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
				nullStr(updates, "display_name"),
				nullStr(updates, "website"),
				nullStr(updates, "owner_id"))
		} else {
			res, err = tx.ExecContext(ctx, `
				UPDATE organization
				SET name       = COALESCE($3, name),
				    website    = COALESCE($4, website),
				    owner_id   = COALESCE($5, owner_id),
				    updated_at = now()
				WHERE id=$1::uuid AND workspace_id=$2::uuid AND version=$6 AND archived_at IS NULL`,
				id, workspaceID,
				nullStr(updates, "display_name"),
				nullStr(updates, "website"),
				nullStr(updates, "owner_id"),
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
		return nil
	})
	if err != nil {
		return Organization{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes an organization (sets archived_at).
func (s *OrgStore) Archive(ctx context.Context, id, workspaceID string) (Organization, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE organization SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		return err
	})
	if err != nil {
		return Organization{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// getAny fetches an organization by id regardless of archived_at status.
//
//nolint:dupl // parallel per-entity CRUD: the SQL column list and Scan targets differ by type; a generic extraction would read worse than the explicit form
func (s *OrgStore) getAny(ctx context.Context, id, workspaceID string) (Organization, error) {
	var o Organization
	var socialRaw, addrRaw []byte
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
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
	unmarshalJSON(socialRaw, &o.Social)
	if addrRaw != nil {
		o.Address = map[string]any{}
		unmarshalJSON(addrRaw, &o.Address)
	}
	return o, nil
}

// attachOrgDomains loads live organization_domain rows for org o and assigns
// them to o.Domains. Called by Get/getAny (the single-record read path) -
// List/listBy* leave Domains nil, matching the contract's "populated on
// getOrganization only" convention already used for relationships/deals/activities.
func attachOrgDomains(ctx context.Context, tx *sql.Tx, workspaceID string, o *Organization) error {
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
	domains := []OrganizationDomain{}
	for rows.Next() {
		var d OrganizationDomain
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
