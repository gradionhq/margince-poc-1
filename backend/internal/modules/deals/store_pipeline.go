package deals

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// ---------------------------------------------------------------------------
// PipelineStore
// ---------------------------------------------------------------------------

// PipelineStore manages pipeline rows.
type PipelineStore struct{ db *sql.DB }

// NewPipelineStore returns a PipelineStore.
func NewPipelineStore(db *sql.DB) *PipelineStore { return &PipelineStore{db: db} }

// Create inserts a pipeline in one workspace-scoped tx.
func (s *PipelineStore) Create(ctx context.Context, pl Pipeline) (Pipeline, error) {
	pl.ID = ids.New()
	err := withWorkspaceTx(ctx, s.db, pl.WorkspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO pipeline (id, workspace_id, name, is_default, position)
			VALUES ($1,$2,$3,$4,$5)`,
			pl.ID, pl.WorkspaceID, pl.Name, pl.IsDefault, pl.Position)
		return err
	})
	if err != nil {
		return Pipeline{}, err
	}
	return s.Get(ctx, pl.ID, pl.WorkspaceID)
}

// Get returns one pipeline by id, workspace-scoped; ErrNotFound if absent.
func (s *PipelineStore) Get(ctx context.Context, id, workspaceID string) (Pipeline, error) {
	var pl Pipeline
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, name, is_default, position, created_at, updated_at, archived_at
			FROM pipeline WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID).Scan(
			&pl.ID, &pl.WorkspaceID, &pl.Name, &pl.IsDefault, &pl.Position,
			&pl.CreatedAt, &pl.UpdatedAt, &pl.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return pl, errs.ErrNotFound
	}
	return pl, err
}

// List returns a keyset page of pipelines for the workspace and the next cursor.
func (s *PipelineStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]Pipeline, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var out []Pipeline
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, name, is_default, position, created_at, updated_at
			FROM pipeline
			WHERE workspace_id=$1::uuid AND archived_at IS NULL
			  AND ($2 = '' OR id::text > $2)
			ORDER BY position, id LIMIT $3`,
			workspaceID, cursor, limit+1)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var pl Pipeline
			if err := rows.Scan(&pl.ID, &pl.WorkspaceID, &pl.Name, &pl.IsDefault, &pl.Position,
				&pl.CreatedAt, &pl.UpdatedAt); err != nil {
				return err
			}
			out = append(out, pl)
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

// Update applies the RC-1 bounded pipeline update surface inside one tx.
func (s *PipelineStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any) (Pipeline, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE pipeline
			SET name       = COALESCE($3, name),
			    position   = COALESCE($4, position),
			    is_default = COALESCE($5, is_default),
			    updated_at = now()
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID, nullStr(updates, "name"), nullInt(updates, "position"), nullBool(updates, "is_default"))
		if err != nil {
			var pgErr *pq.Error
			if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.Constraint == "uq_pipeline_default" {
				return errs.ErrConflict
			}
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errs.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return Pipeline{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes a pipeline (sets archived_at).
func (s *PipelineStore) Archive(ctx context.Context, id, workspaceID string) (Pipeline, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE pipeline SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		return err
	})
	if err != nil {
		return Pipeline{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// getAny fetches a pipeline by id regardless of archived_at status.
func (s *PipelineStore) getAny(ctx context.Context, id, workspaceID string) (Pipeline, error) {
	var pl Pipeline
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, name, is_default, position, created_at, updated_at, archived_at
			FROM pipeline WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(
			&pl.ID, &pl.WorkspaceID, &pl.Name, &pl.IsDefault, &pl.Position,
			&pl.CreatedAt, &pl.UpdatedAt, &pl.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return pl, errs.ErrNotFound
	}
	return pl, err
}

// ---------------------------------------------------------------------------
// StageStore
// ---------------------------------------------------------------------------

// StageStore manages stage rows.
type StageStore struct{ db *sql.DB }

// NewStageStore returns a StageStore.
func NewStageStore(db *sql.DB) *StageStore { return &StageStore{db: db} }

// Create inserts a stage in one workspace-scoped tx.
func (s *StageStore) Create(ctx context.Context, st Stage) (Stage, error) {
	st.ID = ids.New()
	err := withWorkspaceTx(ctx, s.db, st.WorkspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			st.ID, st.WorkspaceID, st.PipelineID, st.Name, st.Position, st.Semantic, st.WinProbability)
		return err
	})
	if err != nil {
		return Stage{}, err
	}
	return s.Get(ctx, st.ID, st.WorkspaceID)
}

// Get returns one stage by id, workspace-scoped; ErrNotFound if absent.
func (s *StageStore) Get(ctx context.Context, id, workspaceID string) (Stage, error) {
	var st Stage
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, pipeline_id, name, position, semantic, win_probability,
			       created_at, updated_at, archived_at
			FROM stage WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID).Scan(
			&st.ID, &st.WorkspaceID, &st.PipelineID, &st.Name, &st.Position, &st.Semantic, &st.WinProbability,
			&st.CreatedAt, &st.UpdatedAt, &st.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return st, errs.ErrNotFound
	}
	return st, err
}

// List returns a keyset page of stages for a pipeline and the next cursor.
func (s *StageStore) List(ctx context.Context, workspaceID, pipelineID, cursor string, limit int) ([]Stage, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var out []Stage
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var rows *sql.Rows
		var err error
		if pipelineID != "" {
			rows, err = tx.QueryContext(ctx, `
				SELECT id, workspace_id, pipeline_id, name, position, semantic, win_probability, created_at, updated_at
				FROM stage
				WHERE workspace_id=$1::uuid AND pipeline_id=$2::uuid AND archived_at IS NULL
				  AND ($3 = '' OR id::text > $3)
				ORDER BY position, id LIMIT $4`,
				workspaceID, pipelineID, cursor, limit+1)
		} else {
			rows, err = tx.QueryContext(ctx, `
				SELECT id, workspace_id, pipeline_id, name, position, semantic, win_probability, created_at, updated_at
				FROM stage
				WHERE workspace_id=$1::uuid AND archived_at IS NULL
				  AND ($2 = '' OR id::text > $2)
				ORDER BY position, id LIMIT $3`,
				workspaceID, cursor, limit+1)
		}
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var st Stage
			if err := rows.Scan(&st.ID, &st.WorkspaceID, &st.PipelineID, &st.Name, &st.Position,
				&st.Semantic, &st.WinProbability, &st.CreatedAt, &st.UpdatedAt); err != nil {
				return err
			}
			out = append(out, st)
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

// Update reorders a stage using SET CONSTRAINTS DEFERRED to avoid transient unique violations.
func (s *StageStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any) (Stage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Stage{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return Stage{}, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return Stage{}, err
	}

	// Defer unique constraints so position reorder doesn't collide mid-transaction.
	// Best-effort: if the constraint isn't deferrable, fall back without deferring.
	_, _ = tx.ExecContext(ctx, `SAVEPOINT stage_update_constraints`)
	if _, err := tx.ExecContext(ctx, `SET CONSTRAINTS uq_stage_position DEFERRED`); err != nil {
		_, _ = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT stage_update_constraints`)
	} else {
		_, _ = tx.ExecContext(ctx, `RELEASE SAVEPOINT stage_update_constraints`)
	}

	setClauses := []string{"updated_at = now()"}
	args := []any{id, workspaceID}
	i := 3
	if name := nullStr(updates, "name"); name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", i))
		args = append(args, *name)
		i++
	}
	if pos := nullInt(updates, "position"); pos != nil {
		setClauses = append(setClauses, fmt.Sprintf("position = $%d", i))
		args = append(args, *pos)
		i++
	}
	if wp := nullInt(updates, "win_probability"); wp != nil {
		setClauses = append(setClauses, fmt.Sprintf("win_probability = $%d", i))
		args = append(args, *wp)
		i++
	}

	//nolint:gosec // G201: setClauses are hardcoded "col = $N" fragments with bound-param indices; all values are passed via args
	q := fmt.Sprintf(`UPDATE stage SET %s WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
		strings.Join(setClauses, ", "))
	res, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		return Stage{}, translateStageUpdateErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Stage{}, errs.ErrNotFound
	}

	var st Stage
	err = tx.QueryRowContext(ctx, `
		SELECT id, workspace_id, pipeline_id, name, position, semantic, win_probability,
		       created_at, updated_at, archived_at
		FROM stage WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		id, workspaceID).Scan(
		&st.ID, &st.WorkspaceID, &st.PipelineID, &st.Name, &st.Position, &st.Semantic, &st.WinProbability,
		&st.CreatedAt, &st.UpdatedAt, &st.ArchivedAt,
	)
	if err != nil {
		return Stage{}, err
	}

	return st, tx.Commit()
}

// translateStageUpdateErr maps stage-table constraint violations from Update to the
// matching Tier-0 sentinel; unrecognized errors pass through unchanged.
func translateStageUpdateErr(err error) error {
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && pgErr.Constraint == "uq_stage_position":
			return errs.ErrConflict
		case pgErr.Code == "23514" && pgErr.Constraint == "stage_terminal_prob":
			return errs.ErrTerminalProbabilityPinned
		case pgErr.Code == "23514" && pgErr.Constraint == "stage_win_probability_check":
			return errs.ErrWinProbabilityOutOfRange
		}
	}
	return err
}

// Archive soft-deletes a stage (sets archived_at).
func (s *StageStore) Archive(ctx context.Context, id, workspaceID string) (Stage, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE stage SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		return err
	})
	if err != nil {
		return Stage{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// getAny fetches a stage by id regardless of archived_at status.
func (s *StageStore) getAny(ctx context.Context, id, workspaceID string) (Stage, error) {
	var st Stage
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, pipeline_id, name, position, semantic, win_probability,
			       created_at, updated_at, archived_at
			FROM stage WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(
			&st.ID, &st.WorkspaceID, &st.PipelineID, &st.Name, &st.Position, &st.Semantic, &st.WinProbability,
			&st.CreatedAt, &st.UpdatedAt, &st.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return st, errs.ErrNotFound
	}
	return st, err
}
