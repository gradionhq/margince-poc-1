package crmcore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// ---------------------------------------------------------------------------
// OrgStore
// ---------------------------------------------------------------------------

// OrgStore executes parameterized SQL against the organization table.
type OrgStore struct{ db *sql.DB }

// NewOrgStore returns an OrgStore.
func NewOrgStore(db *sql.DB) *OrgStore { return &OrgStore{db: db} }

// Create inserts an organization in one workspace-scoped tx.
func (s *OrgStore) Create(ctx context.Context, o Organization) (Organization, error) {
	if err := requireProvenance(o.Source, o.CapturedBy); err != nil {
		return Organization{}, err
	}
	o.ID = ids.New()
	social := marshalJSON(o.Social)
	address := marshalJSON(o.Address)
	err := withWorkspaceTx(ctx, s.db, o.WorkspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO organization (id, workspace_id, name, website, classification, relevance,
			    owner_id, social, address, source, captured_by, version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,1)`,
			o.ID, o.WorkspaceID, o.DisplayName, o.Website, o.Classification, o.Relevance,
			o.OwnerID, social, address,
			o.Source, o.CapturedBy)
		return err
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
		return tx.QueryRowContext(ctx, `
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

// List returns a keyset page of organizations for the workspace and the next cursor.
//
//nolint:dupl // parallel per-entity CRUD: the SQL column list and Scan targets differ by type; a generic extraction would read worse than the explicit form
func (s *OrgStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]Organization, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var out []Organization
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
		return tx.QueryRowContext(ctx, `
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
