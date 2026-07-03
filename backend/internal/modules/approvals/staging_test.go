package crmapprovals_test

import (
	"testing"
	"time"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

func TestStage_DefaultTTL(t *testing.T) {
	if crmapprovals.DefaultApprovalTTL != 72*time.Hour {
		t.Fatalf("DefaultApprovalTTL = %v, want 72h", crmapprovals.DefaultApprovalTTL)
	}
}
