import { useDroppable } from "@dnd-kit/core";
import type { Deal, Stage } from "../../../lib/api-client/generated/index.js";
import { DraggableDealCard, formatMoney, weightedValue } from "./DealCard.js";

export function StageColumn({
  stage,
  deals,
  isTransient = false,
  onCardClick,
  onAdvanceClick,
}: {
  stage: Stage;
  deals: Deal[];
  isTransient?: boolean;
  onCardClick: (dealId: string) => void;
  onAdvanceClick?: (dealId: string) => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: stage.id });
  const raw = deals.reduce((sum, d) => sum + (d.amount_minor ?? 0), 0);
  const weighted = deals.reduce(
    (sum, d) => sum + weightedValue(d.amount_minor, stage.win_probability),
    0,
  );
  const sorted = deals
    .slice()
    .sort((a, b) => (b.amount_minor ?? 0) - (a.amount_minor ?? 0));

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
          {deals.length} deals · {formatMoney(raw, deals[0]?.currency ?? "EUR")}{" "}
          raw · {formatMoney(weighted, deals[0]?.currency ?? "EUR")} weighted
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
          />
        ))}
      </div>
    </div>
  );
}
