import type { PipelineRollup } from "../../../lib/api-client/generated/index.js";
import { Skeleton } from "../../../shared/ui/forge.js";
import { formatMoney } from "./DealCard.js";

export function TotalsStrip({
  rollup,
  isLoading,
  isError,
}: {
  rollup: PipelineRollup | undefined;
  isLoading: boolean;
  isError: boolean;
}) {
  if (isLoading) {
    return (
      <div data-testid="totals-strip-skeleton" className="flex gap-gf-lg p-gf-md">
        <Skeleton height="24px" />
        <Skeleton height="24px" />
        <Skeleton height="24px" />
      </div>
    );
  }
  if (isError || !rollup) {
    return (
      <div className="p-gf-md text-gf-caption text-gf-status-danger">
        Failed to load pipeline totals.
      </div>
    );
  }
  const openDealCount = rollup.by_stage.reduce((n, s) => n + s.deal_count, 0);
  return (
    <div className="flex gap-gf-xl p-gf-md">
      <div>
        <p className="text-gf-caption text-gf-secondary">Weighted</p>
        <p className="text-gf-title font-semibold text-gf-accent">
          {formatMoney(rollup.weighted_minor, rollup.base_currency)}
        </p>
      </div>
      <div>
        <p className="text-gf-caption text-gf-secondary">Raw pipeline</p>
        <p className="text-gf-title font-semibold text-gf-primary">
          {formatMoney(rollup.unweighted_minor, rollup.base_currency)}
        </p>
      </div>
      <div>
        <p className="text-gf-caption text-gf-secondary">Open deals</p>
        <p className="text-gf-title font-semibold text-gf-primary">
          {openDealCount}
        </p>
      </div>
    </div>
  );
}
