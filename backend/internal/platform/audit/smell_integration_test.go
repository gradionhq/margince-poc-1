//go:build integration

package crmaudit_test

import (
	"context"
	"testing"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func nullStrArg(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func TestManualEntrySmell_OverSeededMix(t *testing.T) {
	db := testDB(t)
	wsID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "w"+wsID, "w"+wsID)
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)
	// 2 human, 2 agent activities on channel 'email'; 1 human on NULL channel (-> "direct").
	ins := func(captured, ch string) {
		mustExec(t, db, `INSERT INTO activity (id,workspace_id,kind,source_system,source,captured_by,version)
			VALUES ($1::uuid,$2::uuid,'note',$3,$4,$5,1)`,
			ids.New(), wsID, nullStrArg(ch), "src", captured)
	}
	ins("human:u1", "email")
	ins("human:u2", "email")
	ins("agent:bot", "email")
	ins("agent:bot", "email")
	ins("human:u3", "")

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})
	rows, err := crmaudit.ManualEntrySmell(ctx, db, wsID)
	if err != nil {
		t.Fatalf("smell: %v", err)
	}
	byCh := map[string]crmaudit.SmellRow{}
	for _, r := range rows {
		byCh[r.Channel] = r
	}
	email := byCh["email"]
	if email.Total != 4 || email.Manual != 2 {
		t.Fatalf("email channel: total=%d manual=%d want 4/2", email.Total, email.Manual)
	}
	if email.ManualPct != 0.5 {
		t.Fatalf("email manual_pct=%v want 0.5", email.ManualPct)
	}
	direct := byCh["direct"]
	if direct.Total != 1 || direct.Manual != 1 {
		t.Fatalf("direct channel: total=%d manual=%d want 1/1", direct.Total, direct.Manual)
	}
}
