package crmcore

import "time"

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
