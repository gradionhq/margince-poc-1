package crmapprovals_test

import (
	"testing"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

func TestFlagEgress_PII(t *testing.T) {
	flags := crmapprovals.FlagEgress("Hello John Doe", map[string]string{
		"email":     "john@example.com",
		"full_name": "John Doe",
	})
	if len(flags) == 0 {
		t.Fatal("expected egress flags for pii fields, got none")
	}
	classes := map[string]string{}
	for _, f := range flags {
		classes[f.FieldPath] = f.SensitivityClass
	}
	if classes["email"] != "pii" {
		t.Errorf("email sensitivity_class = %q, want pii", classes["email"])
	}
	if classes["full_name"] != "pii" {
		t.Errorf("full_name sensitivity_class = %q, want pii", classes["full_name"])
	}
}

func TestFlagEgress_Financial(t *testing.T) {
	flags := crmapprovals.FlagEgress("Your revenue report", map[string]string{
		"revenue": "100000",
		"salary":  "50000",
	})
	classes := map[string]string{}
	for _, f := range flags {
		classes[f.FieldPath] = f.SensitivityClass
	}
	if classes["revenue"] != "financial" {
		t.Errorf("revenue sensitivity_class = %q, want financial", classes["revenue"])
	}
	if classes["salary"] != "financial" {
		t.Errorf("salary sensitivity_class = %q, want financial", classes["salary"])
	}
}

func TestFlagEgress_NoFlags(t *testing.T) {
	flags := crmapprovals.FlagEgress("Hello there", map[string]string{
		"notes": "some safe text",
	})
	if len(flags) != 0 {
		t.Errorf("expected no flags for non-sensitive fields, got %d", len(flags))
	}
}

func TestFlagEgress_Empty(t *testing.T) {
	flags := crmapprovals.FlagEgress("", map[string]string{})
	if len(flags) != 0 {
		t.Errorf("expected no flags for empty input, got %d", len(flags))
	}
}

func TestFlagEgress_ResultShape(t *testing.T) {
	flags := crmapprovals.FlagEgress("transfer funds", map[string]string{
		"bank_account": "12345678",
	})
	if len(flags) == 0 {
		t.Fatal("expected flags for bank_account field")
	}
	for _, f := range flags {
		if f.FieldPath == "" {
			t.Error("EgressFlag.FieldPath must not be empty")
		}
		if f.SensitivityClass == "" {
			t.Error("EgressFlag.SensitivityClass must not be empty")
		}
	}
}
