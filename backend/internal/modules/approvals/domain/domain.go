// Package domain contains pure approvals domain types and business-rule functions.
// No database/sql, no net/http — only stdlib.
package domain

import (
	"encoding/json"
	"strings"
	"time"
)

// Status is the lifecycle state of an approval_item row.
type Status string

// The approval_item lifecycle states.
const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusModified Status = "modified"
	StatusExpired  Status = "expired"
)

// Item is one approval_item row.
type Item struct {
	ID                 string
	WorkspaceID        string
	ActionType         string
	Payload            json.RawMessage
	DryRunPreview      json.RawMessage
	TrustTiers         json.RawMessage
	ContentEgressFlags json.RawMessage
	ResumeWindow       json.RawMessage
	Status             Status
	RequestedBy        string
	DecidedBy          *string
	DecidedAt          *time.Time
	ExpiresAt          *time.Time
	CreatedAt          time.Time
}

// ExpiryArgs is the River job payload for the approval expiry sweep.
type ExpiryArgs struct{}

// Kind implements river.JobArgs.
func (ExpiryArgs) Kind() string { return "approval_expiry_sweep" }

// TokenClaims is the ApprovalToken claim set (crm.yaml
// components.schemas.ApprovalToken / APPR-WIRE-1).
type TokenClaims struct {
	JTI           string    `json:"jti"`
	ApprovalID    string    `json:"approval_id"`
	WorkspaceID   string    `json:"workspace_id"`
	PassportID    *string   `json:"passport_id"`
	OnBehalfOf    *string   `json:"on_behalf_of"`
	Tool          string    `json:"tool"`
	DiffHash      string    `json:"diff_hash"`
	TargetVersion *int64    `json:"target_version"`
	Exp           time.Time `json:"exp"`
	SingleUse     bool      `json:"single_use"`
}

// EgressFlag marks a field in a send body as sensitive per §4.4.
type EgressFlag struct {
	FieldPath        string
	SensitivityClass string // classPII | classFinancial | classSpecialCategory | classMasked
}

// The §4.4 sensitivity classes a send-body field can be tagged with.
const (
	classPII             = "pii"
	classSpecialCategory = "special_category"
	classFinancial       = "financial"
	classMasked          = "masked"
)

// sensitivityMap is the §4.4 static sensitivity catalogue.
var sensitivityMap = map[string]string{
	"email":         classPII,
	"full_name":     classPII,
	"name":          classPII,
	"phone":         classPII,
	"address":       classPII,
	"ip_address":    classPII,
	"dob":           classPII,
	"date_of_birth": classPII,
	"ssn":           classSpecialCategory,
	"health":        classSpecialCategory,
	"medical":       classSpecialCategory,
	"religion":      classSpecialCategory,
	"ethnicity":     classSpecialCategory,
	"income":        classFinancial,
	"revenue":       classFinancial,
	"salary":        classFinancial,
	"bank_account":  classFinancial,
	"credit_card":   classFinancial,
	"card_number":   classFinancial,
	"credit":        classFinancial,
	"financial":     classFinancial,
	"password":      classMasked,
	"secret":        classMasked,
	"token":         classMasked,
	"api_key":       classMasked,
}

// FlagEgress scans the send body and fieldMap keys against the §4.4 sensitivity
// map and returns a flag for each matching field.
func FlagEgress(body string, fieldMap map[string]string) []EgressFlag {
	seen := map[string]bool{}
	var flags []EgressFlag

	for field := range fieldMap {
		if class, ok := sensitivityMap[strings.ToLower(field)]; ok {
			key := field + ":" + class
			if !seen[key] {
				seen[key] = true
				flags = append(flags, EgressFlag{FieldPath: field, SensitivityClass: class})
			}
		}
	}

	lower := strings.ToLower(body)
	for keyword, class := range sensitivityMap {
		if _, alreadyFlagged := fieldMap[keyword]; alreadyFlagged {
			continue
		}
		if strings.Contains(lower, keyword) {
			key := keyword + ":" + class
			if !seen[key] {
				seen[key] = true
				flags = append(flags, EgressFlag{FieldPath: keyword, SensitivityClass: class})
			}
		}
	}

	return flags
}
