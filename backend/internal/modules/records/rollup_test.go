package records

import (
	"database/sql"
	"testing"
	"time"
)

// TestCurrentQuarterBounds proves the calendar-quarter boundary math (RD-PARAM-2 / DM-TZ-4):
// given a known instant + IANA zone, currentQuarterBounds returns the correct
// [start, end) quarter window in that zone -- and, critically, respects the zone rather than
// silently computing in UTC.
func TestCurrentQuarterBounds(t *testing.T) {
	utc := time.UTC
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load America/New_York: %v", err)
	}

	tests := []struct {
		name      string
		now       time.Time
		loc       *time.Location
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "Q1 mid-quarter UTC",
			now:       time.Date(2026, time.February, 15, 9, 30, 0, 0, utc),
			loc:       utc,
			wantStart: time.Date(2026, time.January, 1, 0, 0, 0, 0, utc),
			wantEnd:   time.Date(2026, time.April, 1, 0, 0, 0, 0, utc),
		},
		{
			name:      "Q2 mid-quarter UTC",
			now:       time.Date(2026, time.May, 10, 0, 0, 0, 0, utc),
			loc:       utc,
			wantStart: time.Date(2026, time.April, 1, 0, 0, 0, 0, utc),
			wantEnd:   time.Date(2026, time.July, 1, 0, 0, 0, 0, utc),
		},
		{
			name:      "Q3 mid-quarter UTC",
			now:       time.Date(2026, time.August, 20, 23, 59, 0, 0, utc),
			loc:       utc,
			wantStart: time.Date(2026, time.July, 1, 0, 0, 0, 0, utc),
			wantEnd:   time.Date(2026, time.October, 1, 0, 0, 0, 0, utc),
		},
		{
			name:      "Q4 mid-quarter UTC",
			now:       time.Date(2026, time.November, 5, 12, 0, 0, 0, utc),
			loc:       utc,
			wantStart: time.Date(2026, time.October, 1, 0, 0, 0, 0, utc),
			wantEnd:   time.Date(2027, time.January, 1, 0, 0, 0, 0, utc),
		},
		{
			name:      "exactly on a quarter boundary is the start of that quarter (inclusive)",
			now:       time.Date(2026, time.July, 1, 0, 0, 0, 0, utc),
			loc:       utc,
			wantStart: time.Date(2026, time.July, 1, 0, 0, 0, 0, utc),
			wantEnd:   time.Date(2026, time.October, 1, 0, 0, 0, 0, utc),
		},
		{
			// 2026-10-01T02:00:00Z is still 2026-09-30T22:00 in New York (EDT, -04:00) -> Q3,
			// whereas a naive UTC computation would land in Q4. Proves the zone is honored.
			name:      "non-UTC zone straddling a boundary uses the zone, not UTC",
			now:       time.Date(2026, time.October, 1, 2, 0, 0, 0, utc),
			loc:       ny,
			wantStart: time.Date(2026, time.July, 1, 0, 0, 0, 0, ny),
			wantEnd:   time.Date(2026, time.October, 1, 0, 0, 0, 0, ny),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStart, gotEnd := currentQuarterBounds(tc.now, tc.loc)
			if !gotStart.Equal(tc.wantStart) {
				t.Errorf("start = %s, want %s", gotStart, tc.wantStart)
			}
			if !gotEnd.Equal(tc.wantEnd) {
				t.Errorf("end = %s, want %s", gotEnd, tc.wantEnd)
			}
		})
	}
}

// TestNodeReadable exercises every cell of the RD-T04 Architecture-section RBAC readability table:
// all -> always readable; own/team -> owner match, teammate match, live grant, and the
// conservative "NULL owner is NOT auto-included" rule.
func TestNodeReadable(t *testing.T) {
	viewer := sql.NullString{String: "viewer-1", Valid: true}
	otherOwner := sql.NullString{String: "someone-else", Valid: true}
	viewerOwner := sql.NullString{String: "viewer-1", Valid: true}
	nullOwner := sql.NullString{Valid: false}
	teammate := sql.NullString{String: "teammate-9", Valid: true}
	teammates := map[string]bool{"viewer-1": true, "teammate-9": true}

	tests := []struct {
		name      string
		rowScope  string
		ownerID   sql.NullString
		teammates map[string]bool
		hasGrant  bool
		want      bool
	}{
		{"all sees everything regardless of owner", "all", otherOwner, nil, false, true},
		{"all sees a NULL-owner node", "all", nullOwner, nil, false, true},

		{"own + owner is viewer", "own", viewerOwner, nil, false, true},
		{"own + owner is someone else, no grant", "own", otherOwner, nil, false, false},
		{"own + owner is someone else, with grant", "own", otherOwner, nil, true, true},
		{"own + NULL owner, no grant -> excluded (conservative)", "own", nullOwner, nil, false, false},
		{"own + NULL owner, with grant -> included", "own", nullOwner, nil, true, true},
		{"own ignores teammate membership", "own", teammate, teammates, false, false},

		{"team + owner is viewer", "team", viewerOwner, teammates, false, true},
		{"team + owner is a teammate", "team", teammate, teammates, false, true},
		{"team + owner outside team, no grant", "team", otherOwner, teammates, false, false},
		{"team + owner outside team, with grant", "team", otherOwner, teammates, true, true},
		{"team + NULL owner, no grant -> excluded", "team", nullOwner, teammates, false, false},
		{"team + NULL owner, with grant -> included", "team", nullOwner, teammates, true, true},

		{"absent/unknown row_scope is not readable", "", otherOwner, nil, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nodeReadable(tc.rowScope, tc.ownerID, viewer, tc.teammates, tc.hasGrant)
			if got != tc.want {
				t.Errorf("nodeReadable(%q, owner=%+v, grant=%v) = %v, want %v",
					tc.rowScope, tc.ownerID, tc.hasGrant, got, tc.want)
			}
		})
	}
}

// TestSumMinor_ZeroDealsContributesZero proves the RD-FORM-1 edge case at the pure-aggregation
// boundary: a node whose contributing-row slice is empty sums to 0 -- a real, present zero, never
// an omitted/blank field.
func TestSumMinor_ZeroDealsContributesZero(t *testing.T) {
	if got := sumMinor(nil); got != 0 {
		t.Errorf("sumMinor(nil) = %d, want 0", got)
	}
	if got := sumMinor([]int64{}); got != 0 {
		t.Errorf("sumMinor(empty) = %d, want 0", got)
	}
	if got := sumMinor([]int64{125, 375, 0}); got != 500 {
		t.Errorf("sumMinor([125,375,0]) = %d, want 500", got)
	}
}
