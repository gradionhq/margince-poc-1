import { Skeleton } from "../../../shared/ui/forge.js";
import type { OrganizationHierarchyRollup } from "../api/records.js";

export function formatMoneyDeDE(
  amountMinor: number | null | undefined,
  currency: string | null | undefined,
): string {
  if (amountMinor == null || !currency) return "—";
  return new Intl.NumberFormat("de-DE", {
    style: "currency",
    currency,
  }).format(amountMinor / 100);
}

export function RollupTilesBand({
  rollup,
  isLoading,
  isError,
  depth,
  nodeCount,
}: {
  rollup: OrganizationHierarchyRollup | undefined;
  isLoading: boolean;
  isError: boolean;
  depth: number;
  nodeCount: number;
}) {
  if (isLoading) {
    return (
      <div
        data-testid="rollup-tiles-band-skeleton"
        className="flex gap-gf-lg p-gf-md"
      >
        <Skeleton height="24px" />
        <Skeleton height="24px" />
        <Skeleton height="24px" />
      </div>
    );
  }
  if (isError || !rollup) {
    return (
      <div className="p-gf-md text-gf-caption text-gf-status-danger">
        Failed to load hierarchy roll-up.
      </div>
    );
  }

  const budgetPct = Math.round((nodeCount / 200) * 100);

  return (
    <div className="p-gf-md">
      <div className="flex gap-gf-xl">
        <div>
          <p className="text-gf-caption text-gf-secondary">Weighted Pipeline</p>
          <p className="text-gf-title font-semibold text-gf-accent">
            {formatMoneyDeDE(
              rollup.weighted_pipeline.amount_minor,
              rollup.weighted_pipeline.currency,
            )}
          </p>
        </div>
        <div>
          <p className="text-gf-caption text-gf-secondary">Closed-Won FY26</p>
          <p className="text-gf-title font-semibold text-gf-primary">
            {formatMoneyDeDE(
              rollup.closed_won.amount_minor,
              rollup.closed_won.currency,
            )}
          </p>
        </div>
        <div>
          <p className="text-gf-caption text-gf-secondary">30-Day Activity</p>
          <p className="text-gf-title font-semibold text-gf-primary">
            {rollup.activity_count_30d}
          </p>
        </div>
      </div>
      <p className="mt-gf-xs text-gf-caption text-gf-secondary">
        Aggregated over {rollup.aggregated_account_count} accounts
      </p>
      <p className="mt-gf-xs text-gf-caption text-gf-muted">
        depth {depth} · {nodeCount} nodes · {budgetPct}% of P11 budget
      </p>
      <p className="mt-gf-xs text-gf-caption text-gf-muted">
        EUR · ISO-4217 · integer minor-units
      </p>
    </div>
  );
}
