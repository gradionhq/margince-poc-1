package crmcore

import (
	"context"
	"database/sql"
)

// orgDedupeCandidate is one row from the org candidate-set query.
type orgDedupeCandidate struct {
	id   string
	name string
}

// orgDedupeCandidates returns every live organization in workspaceID
// (excluding excludeID) sharing a name trigram with normalizedName — the
// coarse prefilter (PO-F-2 has no org-match ladder, name-only).
func orgDedupeCandidates(ctx context.Context, tx *sql.Tx, workspaceID, excludeID, normalizedName string) ([]orgDedupeCandidate, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, name FROM organization
		WHERE workspace_id=$1::uuid AND archived_at IS NULL AND id <> $2::uuid
		  AND lower(name) % $3
		ORDER BY id`,
		workspaceID, excludeID, normalizedName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []orgDedupeCandidate
	for rows.Next() {
		var c orgDedupeCandidate
		if err := rows.Scan(&c.id, &c.name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// fuzzyDedupe is PO-F-2's name-only fuzzy tier, run inside OrgStore.Create's
// tx after the exact-domain tier has already succeeded. confidence is the
// legal-suffix-normalized Jaro-Winkler name_sim alone (no org-match term —
// an org has no "org" to match against). Ties resolve to the lowest org id.
func (s *OrgStore) fuzzyDedupe(ctx context.Context, tx *sql.Tx, workspaceID, excludeID, displayName string) (*DedupeReviewFlag, error) {
	normalizedName := normalizeCompanyName(displayName)
	if normalizedName == "" {
		return nil, nil //nolint:nilnil // empty name skips the fuzzy tier entirely — nil flag + nil error is the "no review" result
	}
	candidates, err := orgDedupeCandidates(ctx, tx, workspaceID, excludeID, normalizedName)
	if err != nil {
		return nil, err
	}
	var best *DedupeReviewFlag
	for _, c := range candidates {
		confidence := jaroWinkler(normalizedName, normalizeCompanyName(c.name))
		if best == nil || confidence > best.Confidence {
			best = &DedupeReviewFlag{CandidateID: c.id, Confidence: confidence}
		}
	}
	if best == nil || best.Confidence < dedupeReviewThreshold {
		return nil, nil //nolint:nilnil // no candidate cleared the threshold — nil flag + nil error is the "no review" result
	}
	return best, nil
}
