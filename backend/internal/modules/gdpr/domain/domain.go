// Package domain contains pure GDPR domain types and business-rule functions.
// No database/sql, no net/http — only stdlib and the spec invariants.
package domain

import (
	"encoding/json"
	"time"
)

// ConsentState is the current consent status for a given (person, purpose) pair.
type ConsentState string

// The consent states a (person, purpose) pair can be in.
const (
	Granted   ConsentState = "granted"
	Withdrawn ConsentState = "withdrawn"
	Unknown   ConsentState = "unknown"
)

// SARPackage holds all data held about a subject for an Art. 15 Subject Access Request.
type SARPackage struct {
	Person        json.RawMessage
	Emails        []json.RawMessage
	Activities    []json.RawMessage
	Deals         []json.RawMessage
	Organizations []json.RawMessage
	RawCapture    []json.RawMessage
}

// Policy is a retention rule for an object type + optional category.
type Policy struct {
	ObjectType string
	Category   string
	RetainDays int
	Action     string
}

// RetentionSweepArgs is the payload for the nightly retention sweep job.
type RetentionSweepArgs struct{}

// Kind implements river.JobArgs.
func (RetentionSweepArgs) Kind() string { return "retention_sweep" }

// OverAge reports whether lastActivity is more than retainDays×24h before asOf.
// Exactly retainDays is NOT over age; one second beyond is.
func OverAge(asOf time.Time, retainDays int, lastActivity time.Time) bool {
	threshold := time.Duration(retainDays) * 24 * time.Hour
	return asOf.Sub(lastActivity) > threshold
}
