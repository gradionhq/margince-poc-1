// Package dedupe provides dependency-free fuzzy-match normalization and
// Jaro-Winkler scoring used by the person and organization dedupe pipelines
// (PO-F-1, PO-PARAM-JW-1/JW-2). Pure functions — no DB/context access (WS-E-b, D5).
package dedupe

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Named source constants — no runtime-config surface, no admin-tunable UI
// (per ticket: all five tunables are hand-fixed, not contract/config fields).
const (
	// DedupeReviewThreshold is the minimum fuzzy confidence score that triggers
	// a non-blocking PO-AC-19 ReviewFlag on Create (DEDUPE_REVIEW_THRESHOLD).
	DedupeReviewThreshold = 0.72
	dedupeNameWeight      = 0.55 // DEDUPE_NAME_WEIGHT (internal — used in PersonConfidence)
	dedupeOrgWeight       = 0.45 // DEDUPE_ORGDOMAIN_WEIGHT (internal)
	jwPrefixScale         = 0.1  // PO-PARAM-JW-1 (internal)
	jwMaxPrefix           = 4    // PO-PARAM-JW-1 (internal)
)

// legalSuffixes is PO-PARAM-1's fixed, case-insensitive, trailing-only company
// legal-suffix list.
var legalSuffixes = map[string]bool{
	"inc": true, "llc": true, "ltd": true, "gmbh": true, "ag": true, "sa": true,
	"sas": true, "bv": true, "oy": true, "plc": true, "co": true, "corp": true,
	"kg": true, "ug": true,
}

// NormalizeName is PO-PARAM-JW-2's casefold+unaccent+trim pipeline:
// lower(trim(unaccent(s))). NFKD-decomposes s and strips Unicode combining
// marks (the idiomatic Go unaccent recipe), then lowercases. No legal-suffix
// strip — this is for person names, use NormalizeCompanyName for companies.
func NormalizeName(s string) string {
	s = strings.TrimSpace(s)
	decomposed := norm.NFKD.String(s)
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue // strip combining diacritical marks
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// NormalizeCompanyName applies NormalizeName, then strips a trailing legal
// suffix (PO-PARAM-1), repeatedly (so "Acme Ltd Co" -> "acme"). Only the
// trailing token is ever stripped, never an interior one.
func NormalizeCompanyName(s string) string {
	fields := strings.Fields(NormalizeName(s))
	for len(fields) > 1 {
		last := strings.Trim(fields[len(fields)-1], ".,")
		if !legalSuffixes[last] {
			break
		}
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

// jaroMatchWindow is the classic Jaro match distance: floor(max(len1,len2)/2)-1,
// clamped at 0.
func jaroMatchWindow(len1, len2 int) int {
	w := len1
	if len2 > w {
		w = len2
	}
	w = w/2 - 1
	if w < 0 {
		w = 0
	}
	return w
}

// jaroMatches marks, for each rune of s1, whether it has a match within
// matchDistance in s2 (and vice versa), returning the two match masks and the
// total match count.
func jaroMatches(s1, s2 []rune, matchDistance int) (s1Matches, s2Matches []bool, matches int) {
	s1Matches = make([]bool, len(s1))
	s2Matches = make([]bool, len(s2))
	for i := range s1 {
		start := i - matchDistance
		if start < 0 {
			start = 0
		}
		end := i + matchDistance + 1
		if end > len(s2) {
			end = len(s2)
		}
		for j := start; j < end; j++ {
			if s2Matches[j] || s1[i] != s2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}
	return s1Matches, s2Matches, matches
}

// jaroTranspositions counts the (halved) transpositions between the matched
// runes of s1 and s2, walking both match masks in order.
func jaroTranspositions(s1, s2 []rune, s1Matches, s2Matches []bool) int {
	k := 0
	transpositions := 0
	for i := range s1 {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if s1[i] != s2[k] {
			transpositions++
		}
		k++
	}
	return transpositions / 2
}

// jaroDistance is the classic Jaro distance (not Winkler-boosted) between two
// rune sequences.
func jaroDistance(s1, s2 []rune) float64 {
	len1, len2 := len(s1), len(s2)
	if len1 == 0 && len2 == 0 {
		return 1.0
	}
	if len1 == 0 || len2 == 0 {
		return 0.0
	}
	s1Matches, s2Matches, matches := jaroMatches(s1, s2, jaroMatchWindow(len1, len2))
	if matches == 0 {
		return 0.0
	}
	transpositions := jaroTranspositions(s1, s2, s1Matches, s2Matches)
	m := float64(matches)
	return (m/float64(len1) + m/float64(len2) + (m-float64(transpositions))/m) / 3.0
}

// JaroWinkler is PO-PARAM-JW-1's standard variant: Jaro distance plus a
// common-prefix boost (prefix scale p=0.1, max prefix length 4), no boost
// threshold (the boost applies unconditionally, unlike some variants that
// only boost above jaro>=0.7).
func JaroWinkler(a, b string) float64 {
	r1, r2 := []rune(a), []rune(b)
	jaro := jaroDistance(r1, r2)
	maxPrefix := len(r1)
	if len(r2) < maxPrefix {
		maxPrefix = len(r2)
	}
	if maxPrefix > jwMaxPrefix {
		maxPrefix = jwMaxPrefix
	}
	prefix := 0
	for i := 0; i < maxPrefix; i++ {
		if r1[i] != r2[i] {
			break
		}
		prefix++
	}
	return jaro + float64(prefix)*jwPrefixScale*(1-jaro)
}

// PersonConfidence is PO-F-1's Tier-2 formula:
// confidence = DEDUPE_NAME_WEIGHT*name_sim + DEDUPE_ORGDOMAIN_WEIGHT*org_match.
func PersonConfidence(nameSim, orgMatch float64) float64 {
	return dedupeNameWeight*nameSim + dedupeOrgWeight*orgMatch
}

// OrgMatchScore is PO-F-1's org-match ladder for scoring a new person against
// an existing candidate person. Returns a score 0.0–1.0.
func OrgMatchScore(newDomainOrgID, candCurrentOrgID, candDomainOrgID *string, newCompany, candCompany string) float64 {
	if newDomainOrgID != nil && candCurrentOrgID != nil && *newDomainOrgID == *candCurrentOrgID {
		return 1.0
	}
	if newDomainOrgID != nil && candDomainOrgID != nil && *newDomainOrgID == *candDomainOrgID {
		return 0.8
	}
	if newCompany != "" && candCompany != "" && NormalizeCompanyName(newCompany) == NormalizeCompanyName(candCompany) {
		return 0.5
	}
	return 0.0
}

// ReviewFlag is the non-blocking PO-AC-19 review-flag attached to a
// Create response when the fuzzy tier's best candidate scores >=
// DedupeReviewThreshold. NEVER persisted — computed fresh on every Create call.
// Fuzzy scoring never auto-merges at any confidence (DEDUPE_FUZZY_AUTOMERGE is
// unconditionally "never").
type ReviewFlag struct {
	CandidateID string  `json:"candidate_id"`
	Confidence  float64 `json:"confidence"`
}
