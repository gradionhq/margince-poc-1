import type { Deal, Stage } from "../../../lib/api-client/generated/index.js";
import { DataTable } from "../../../shared/ui/DataTable.js";
import { formatMoney, stalledDays, weightedValue } from "./DealCard.js";

export function DealsTable({
  deals,
  stagesById,
}: {
  deals: Deal[];
  stagesById: Record<string, Stage>;
}) {
  const sorted = deals.slice().sort((a, b) => (b.amount_minor ?? 0) - (a.amount_minor ?? 0));
  return (
    <DataTable
      rows={sorted}
      getRowKey={(d) => d.id}
      columns={[
        // listDeals rows carry organization_id, not the org name — the contract has no
        // denormalized name to display here without an extra join this ticket doesn't add.
        { key: "company", header: "Company", render: () => "—" },
        { key: "deal", header: "Deal", render: (d) => d.name },
        {
          key: "stage",
          header: "Stage",
          render: (d) => stagesById[d.stage_id]?.name ?? "—",
        },
        {
          key: "prob",
          header: "Prob.",
          render: (d) => `${stagesById[d.stage_id]?.win_probability ?? 0}%`,
        },
        {
          key: "amount",
          header: "Amount",
          render: (d) => formatMoney(d.amount_minor, d.currency),
        },
        {
          key: "weighted",
          header: "Weighted",
          render: (d) =>
            formatMoney(
              weightedValue(d.amount_minor, stagesById[d.stage_id]?.win_probability ?? 0),
              d.currency,
            ),
        },
        {
          key: "age",
          header: "Age",
          render: (d) => (
            <span
              data-testid={`age-cell-${d.id}`}
              className={d.stalled ? "text-gf-status-warning font-medium" : "text-gf-secondary"}
            >
              {d.stalled && "⚠ "}
              {stalledDays(d.stage_entered_at)}d
            </span>
          ),
        },
      ]}
    />
  );
}
