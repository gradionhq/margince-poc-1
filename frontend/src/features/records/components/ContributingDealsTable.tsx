import { DataTable } from "../../../shared/ui/DataTable.js";
import { StatusBadge } from "../../../shared/ui/forge.js";
import type { QuotaAttainment } from "../api/quotas.js";
import { useContributingDealDetails } from "../api/quotas.js";
import { formatMoneyDeDE } from "./RollupTilesBand.js";

const ClosedWonBadge =
  StatusBadge ??
  (({ label }: { label: string; variant?: string }) => (
    <span className="rounded-full border border-gf-accent bg-gf-accent-light px-gf-xs py-[2px] text-gf-caption text-gf-accent">
      {label}
    </span>
  ));

type Row = {
  deal_id: string;
  base_value_minor: number;
  name?: string;
  closedAt?: string | null;
};

export function ContributingDealsTable({
  attainment,
}: {
  attainment: QuotaAttainment;
}) {
  const dealIds = attainment.contributing_deals.map((deal) => deal.deal_id);
  const details = useContributingDealDetails(dealIds);

  const rows: Row[] = attainment.contributing_deals.map((deal) => {
    const detail = details.find((item) => item.id === deal.deal_id);
    return {
      ...deal,
      name: detail?.data?.name,
      closedAt: detail?.data?.closed_at,
    };
  });

  return (
    <div>
      <div className="mb-gf-md flex flex-wrap items-center justify-between gap-gf-sm">
        <h3 className="text-gf-body font-medium text-gf-secondary">
          What counts toward attainment
        </h3>
        <span className="text-gf-caption text-gf-tertiary">
          closed-won deals · clean core
        </span>
      </div>
      <DataTable<Row>
        columns={[
          {
            key: "deal",
            header: "Deal",
            render: (row) => row.name ?? row.deal_id,
          },
          {
            key: "closed",
            header: "Closed",
            render: (row) => (row.closedAt ? row.closedAt.slice(0, 10) : "—"),
          },
          {
            key: "status",
            header: "Status",
            render: () => <ClosedWonBadge label="Closed-won" variant="success" />,
          },
          {
            key: "amount",
            header: "Counted amount",
            render: (row) => formatMoneyDeDE(row.base_value_minor, attainment.currency),
          },
        ]}
        rows={rows}
        getRowKey={(row) => row.deal_id}
      />
      <div className="mt-gf-md flex items-center justify-between border-t-2 border-gf-subtle pt-gf-md">
        <span className="text-gf-body font-semibold text-gf-primary">
          Counted total
        </span>
        <span className="font-mono text-gf-title font-semibold text-gf-primary">
          {formatMoneyDeDE(attainment.closed_won_minor, attainment.currency)}
        </span>
      </div>
      <p className="mt-gf-xs text-right font-mono text-gf-micro text-gf-tertiary">
        {attainment.currency} · amounts to the cent · open / lost / omitted deals excluded
      </p>
    </div>
  );
}
