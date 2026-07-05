import type { Deal, Stage } from "../../../lib/api-client/generated/index.js";
import { ContextMenu } from "../../../shared/ui/ContextMenu.js";
import { DataTable } from "../../../shared/ui/DataTable.js";
import { Icon, IconButton } from "../../../shared/ui/forge.js";
import {
  formatMoney,
  idleDays,
  stalledDays,
  weightedValue,
} from "./DealCard.js";

export function DealsTable({
  deals,
  stagesById,
  onArchive,
}: {
  deals: Deal[];
  stagesById: Record<string, Stage>;
  onArchive?: (dealId: string) => void;
}) {
  const sorted = deals
    .slice()
    .sort((a, b) => (b.amount_minor ?? 0) - (a.amount_minor ?? 0));
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
              weightedValue(
                d.amount_minor,
                stagesById[d.stage_id]?.win_probability ?? 0,
              ),
              d.currency,
            ),
        },
        {
          key: "age",
          header: "Age",
          render: (d) => (
            <span
              data-testid={`age-cell-${d.id}`}
              className={
                d.stalled
                  ? "inline-flex items-center gap-gf-xs text-gf-status-warning font-medium"
                  : "text-gf-secondary"
              }
            >
              {d.stalled && <Icon name="AlertCircle" size={14} />}
              {d.stalled
                ? idleDays(d.last_activity_at, d.created_at)
                : stalledDays(d.stage_entered_at)}
              d
            </span>
          ),
        },
        {
          key: "actions",
          header: "",
          render: (d) => (
            <ContextMenu
              trigger={
                <IconButton
                  icon="MoreVertical"
                  label="Row actions"
                  onClick={(e) => e.stopPropagation()}
                  onKeyDown={(e) => e.stopPropagation()}
                />
              }
              items={[
                {
                  id: "archive",
                  label: "Archive",
                  onSelect: () => onArchive?.(d.id),
                },
              ]}
            />
          ),
        },
      ]}
    />
  );
}
