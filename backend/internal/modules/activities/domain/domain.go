// Package domain holds the Activity entity and its lightweight reference type.
package domain

import (
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// Activity is a timeline event linked to people/orgs/deals (data-model §7).
type Activity struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	Kind            string     `json:"kind"` // email | call | meeting | note | task | whatsapp | telegram
	Subject         *string    `json:"subject"`
	Body            *string    `json:"body"`
	OccurredAt      time.Time  `json:"occurred_at"`
	DueAt           *time.Time `json:"due_at"`
	AssigneeID      *string    `json:"assignee_id"`
	RemindAt        *time.Time `json:"remind_at"`
	IsDone          bool       `json:"is_done"`
	DoneAt          *time.Time `json:"done_at"`
	DurationSeconds *int       `json:"duration_seconds"`
	Direction       *string    `json:"direction"` // inbound | outbound
	MeetingStatus   *string    `json:"meeting_status"`
	SourceSystem    *string    `json:"source_system"`
	SourceID        *string    `json:"source_id"`
	TranscriptRef   *string    `json:"transcript_ref"`
	Version         int64      `json:"version"`
	Source          string     `json:"source"`
	CapturedBy      string     `json:"captured_by"`
	// Provenance is kept for internal use; not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// ActivityRef is a lightweight activity identity reference for composite reads.
type ActivityRef struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Subject    *string   `json:"subject"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ToActivityRef narrows a full Activity row to the fields composite reads carry.
func ToActivityRef(a Activity) ActivityRef {
	return ActivityRef{ID: a.ID, Kind: a.Kind, Subject: a.Subject, OccurredAt: a.OccurredAt}
}

// NewActivity returns an Activity with a fresh ID, version 1, and copied provenance.
func NewActivity(kind string, p prov.Provenance) Activity {
	now := time.Now().UTC()
	return Activity{
		ID: ids.New(), Kind: kind, OccurredAt: now,
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}
