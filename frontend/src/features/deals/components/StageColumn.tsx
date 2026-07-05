import { useDroppable } from "@dnd-kit/core";
import type {
  Deal,
  PipelineRollupStage,
  Stage,
} from "../../../lib/api-client/generated/index.js";
import { DraggableDealCard, formatMoney } from "./DealCard.js";

export function StageColumn({
  stage,
  deals,
  rollupStage,
  baseCurrency,
  isTransient = false,
  onCardClick,
  onAdvanceClick,
  onArchive,
}: {
  stage: Stage;
  deals: Deal[];
  // The per-stage raw/weighted decomposition from GET /pipelines/{id}/rollup
  // (PipelineRollupStage) — the contract is explicit that this is "a server
  // read only — the client never sums these itself" (DEAL-EXT-1). Summing
  // deal.amount_minor client-side here would also silently mix currencies
  // when a column holds deals in more than one currency; the rollup is
  // already reduced to the workspace's base currency. undefined while the
  // rollup hasn't loaded yet (or errored) — rendered as an honest "—", never
  // a fabricated total.
  rollupStage?: PipelineRollupStage;
  baseCurrency?: string;
  isTransient?: boolean;
  onCardClick: (dealId: string) => void;
  onAdvanceClick?: (dealId: string) => void;
  onArchive?: (dealId: string) => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: stage.id });
  const sorted = deals
    .slice()
    .sort((a, b) => (b.amount_minor ?? 0) - (a.amount_minor ?? 0));
  const dealCount = rollupStage?.deal_count ?? deals.length;

  return (
    <div
      ref={setNodeRef}
      data-testid={`stage-column-${stage.id}`}
      className={`flex flex-col min-w-[260px] rounded-lg border p-gf-sm gap-gf-sm ${
        isTransient
          ? "border-dashed border-gf-accent bg-gf-accent-light"
          : "border-gf-subtle"
      } ${isOver ? "ring-2 ring-gf-accent" : ""}`}
    >
      <div className="px-gf-xs">
        <p className="text-gf-body font-semibold text-gf-primary">
          {stage.name} · {stage.win_probability}%
        </p>
        <p className="text-gf-caption text-gf-secondary">
          {dealCount} deal{dealCount === 1 ? "" : "s"} ·{" "}
          {rollupStage && baseCurrency
            ? formatMoney(rollupStage.unweighted_minor, baseCurrency)
            : "—"}{" "}
          raw ·{" "}
          {rollupStage && baseCurrency
            ? formatMoney(rollupStage.weighted_minor, baseCurrency)
            : "—"}{" "}
          weighted
        </p>
      </div>
      <div className="flex flex-col gap-gf-sm min-h-[80px]">
        {sorted.length === 0 && (
          <p className="text-gf-caption text-gf-muted p-gf-sm">
            Drop a card here
          </p>
        )}
        {sorted.map((deal) => (
          <DraggableDealCard
            key={deal.id}
            deal={deal}
            onClick={() => onCardClick(deal.id)}
            onAdvanceClick={onAdvanceClick}
            onArchive={onArchive}
          />
        ))}
      </div>
    </div>
  );
}
