package crmapprovals_test

import (
	"testing"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

func TestStatusConstants(t *testing.T) {
	if crmapprovals.StatusPending != "pending" {
		t.Fatalf("pending mismatch: %q", crmapprovals.StatusPending)
	}
	if crmapprovals.NewRepository() == nil {
		t.Fatal("NewRepository returned nil")
	}
}
