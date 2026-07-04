package crmcore

import (
	"context"
	"database/sql"
	"strings"
)

// personDedupeCandidate is one row from the candidate-set query: a live
// person in the same workspace, restricted to those sharing a name trigram
// or sharing the new person's domain-derived org (never `lead` — leads live
// in a structurally separate table, never selected from here).
type personDedupeCandidate struct {
	id       string
	fullName string
}

// personEmailDomains extracts the lowercased domain half of each input
// email, skipping malformed entries (no '@').
func personEmailDomains(emails []PersonEmailInput) []string {
	domains := make([]string, 0, len(emails))
	for _, e := range emails {
		parts := strings.SplitN(strings.ToLower(strings.TrimSpace(e.Email)), "@", 2)
		if len(parts) == 2 && parts[1] != "" {
			domains = append(domains, parts[1])
		}
	}
	return domains
}

// domainOrgID looks up the first of domains that resolves to a live
// organization_domain row in workspaceID, or nil if none resolve.
func domainOrgID(ctx context.Context, tx *sql.Tx, workspaceID string, domains []string) (*string, error) {
	for _, d := range domains {
		var orgID string
		err := tx.QueryRowContext(ctx, `
			SELECT organization_id FROM organization_domain
			WHERE workspace_id=$1::uuid AND domain=$2 AND archived_at IS NULL`,
			workspaceID, d).Scan(&orgID)
		if err == nil {
			return &orgID, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}
	return nil, nil //nolint:nilnil // no domain resolved to an org — nil org id + nil error is a valid "no signal" result
}

// candidateCurrentOrgID returns personID's live current-primary employment
// org, or nil if it has none.
func candidateCurrentOrgID(ctx context.Context, tx *sql.Tx, workspaceID, personID string) (*string, error) {
	var orgID string
	err := tx.QueryRowContext(ctx, `
		SELECT organization_id FROM relationship
		WHERE workspace_id=$1::uuid AND kind='employment' AND is_primary
		  AND archived_at IS NULL AND person_id=$2::uuid
		LIMIT 1`,
		workspaceID, personID).Scan(&orgID)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // no current-primary employment — nil org id + nil error is a valid "no signal" result
	}
	if err != nil {
		return nil, err
	}
	return &orgID, nil
}

// candidateDomainOrgID returns personID's own domain-derived org (via its
// live emails matched against organization_domain), or nil if none resolve.
func candidateDomainOrgID(ctx context.Context, tx *sql.Tx, workspaceID, personID string) (*string, error) {
	var orgID string
	err := tx.QueryRowContext(ctx, `
		SELECT od.organization_id
		FROM person_email pe
		JOIN organization_domain od ON od.workspace_id = pe.workspace_id
		  AND od.domain = split_part(pe.email, '@', 2) AND od.archived_at IS NULL
		WHERE pe.workspace_id=$1::uuid AND pe.person_id=$2::uuid AND pe.archived_at IS NULL
		LIMIT 1`,
		workspaceID, personID).Scan(&orgID)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // no email domain resolved to an org — nil org id + nil error is a valid "no signal" result
	}
	if err != nil {
		return nil, err
	}
	return &orgID, nil
}

// personDedupeCandidates returns every live person in workspaceID (excluding
// excludeID) sharing a name trigram with normalizedName or sharing newOrgID
// (via current-primary employment or its own domain-derived org) — the
// coarse prefilter that keeps this inside the create budget. Only ever
// selects from `person` — leads (ADR-0008) are a structurally separate
// table, never referenced here.
func personDedupeCandidates(ctx context.Context, tx *sql.Tx, workspaceID, excludeID, normalizedName string, newOrgID *string) ([]personDedupeCandidate, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT p.id, p.full_name
		FROM person p
		WHERE p.workspace_id=$1::uuid AND p.archived_at IS NULL AND p.id <> $2::uuid
		  AND (
		    lower(p.full_name) % $3
		    OR EXISTS (
		      SELECT 1 FROM relationship r
		      WHERE r.workspace_id = p.workspace_id AND r.kind='employment' AND r.is_primary
		        AND r.archived_at IS NULL AND r.person_id = p.id AND r.organization_id = $4::uuid
		    )
		    OR EXISTS (
		      SELECT 1 FROM person_email pe
		      JOIN organization_domain od ON od.workspace_id = pe.workspace_id
		        AND od.domain = split_part(pe.email, '@', 2) AND od.archived_at IS NULL
		      WHERE pe.workspace_id = p.workspace_id AND pe.person_id = p.id
		        AND pe.archived_at IS NULL AND od.organization_id = $4::uuid
		    )
		  )
		ORDER BY p.id`,
		workspaceID, excludeID, normalizedName, newOrgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []personDedupeCandidate
	for rows.Next() {
		var c personDedupeCandidate
		if err := rows.Scan(&c.id, &c.fullName); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// fuzzyDedupe is PO-F-1's Tier-2 scorer, run inside PersonStore.Create's tx
// after the exact-key email tier has already succeeded. Returns nil (no
// review flag) when fullName is empty (edge case: fuzzy tier skipped
// entirely) or no live candidate clears dedupeReviewThreshold. Ties resolve
// to the lowest person id — candidates are iterated in id-ascending order
// and a strict `>` keeps the first (lowest-id) winner on an exact tie.
func (s *PersonStore) fuzzyDedupe(ctx context.Context, tx *sql.Tx, workspaceID, excludeID, fullName string, emails []PersonEmailInput) (*DedupeReviewFlag, error) {
	normalizedName := normalizeName(fullName)
	if normalizedName == "" {
		return nil, nil //nolint:nilnil // empty name skips the fuzzy tier entirely — nil flag + nil error is the "no review" result
	}
	newOrgID, err := domainOrgID(ctx, tx, workspaceID, personEmailDomains(emails))
	if err != nil {
		return nil, err
	}
	candidates, err := personDedupeCandidates(ctx, tx, workspaceID, excludeID, normalizedName, newOrgID)
	if err != nil {
		return nil, err
	}
	var best *DedupeReviewFlag
	for _, c := range candidates {
		candCurrentOrgID, err := candidateCurrentOrgID(ctx, tx, workspaceID, c.id)
		if err != nil {
			return nil, err
		}
		candDomainOrgID, err := candidateDomainOrgID(ctx, tx, workspaceID, c.id)
		if err != nil {
			return nil, err
		}
		nameSim := jaroWinkler(normalizedName, normalizeName(c.fullName))
		orgMatch := orgMatchScore(newOrgID, candCurrentOrgID, candDomainOrgID, "", "")
		confidence := personConfidence(nameSim, orgMatch)
		if best == nil || confidence > best.Confidence {
			best = &DedupeReviewFlag{CandidateID: c.id, Confidence: confidence}
		}
	}
	if best == nil || best.Confidence < dedupeReviewThreshold {
		return nil, nil //nolint:nilnil // no candidate cleared the threshold — nil flag + nil error is the "no review" result
	}
	return best, nil
}
