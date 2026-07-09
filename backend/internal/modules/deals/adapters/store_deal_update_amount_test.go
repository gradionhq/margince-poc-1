//go:build integration

package adapters_test

import (
	"context"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const dealUpdateAmountTestWorkspaceID = "00000000-0000-0000-0000-000000000007"

func TestDealStore_Update_SyncsAmountMinorAndCurrency(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	tag := time.Now().Format("20060102150405.000000000")
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t08-deal-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, dealUpdateAmountTestWorkspaceID, "t08-deal-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, dealUpdateAmountTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}

	var pipelineID, stageID string
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		dealUpdateAmountTestWorkspaceID, "P-"+tag).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position) VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`,
		dealUpdateAmountTestWorkspaceID, pipelineID, "S-"+tag).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}

	store := adapters.NewDealStore(db)
	d := domain.NewDeal("Deal for amount sync", pipelineID, stageID, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = dealUpdateAmountTestWorkspaceID
	created, err := store.Create(context.Background(), d, "", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.AmountMinor != nil {
		t.Fatalf("expected nil amount_minor on a freshly created deal, got %v", created.AmountMinor)
	}

	amountMinor := int64(987654)
	currency := "USD"
	updated, err := store.Update(context.Background(), created.ID, dealUpdateAmountTestWorkspaceID, map[string]any{
		"amount_minor": amountMinor,
		"currency":     currency,
	}, 0)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.AmountMinor == nil || *updated.AmountMinor != amountMinor {
		t.Fatalf("expected amount_minor=%d, got %v", amountMinor, updated.AmountMinor)
	}
	if updated.Currency == nil || *updated.Currency != currency {
		t.Fatalf("expected currency=%s, got %v", currency, updated.Currency)
	}
}
