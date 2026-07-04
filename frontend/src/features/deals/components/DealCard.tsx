import { useDraggable } from "@dnd-kit/core";
import type { Deal } from "../../../lib/api-client/generated/index.js";

export function formatMoney(
  amountMinor: number | null | undefined,
  currency: string | null | undefined,
): string {
  if (amountMinor == null || !currency) return "—";
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
  }).format(amountMinor / 100);
}

// half away from zero, matching DEAL-FORM-2's per-deal rounding rule
export function weightedValue(
  amountMinor: number | null | undefined,
  winProbability: number,
): number {
  if (amountMinor == null) return 0;
  const raw = (amountMinor * winProbability) / 100;
  return raw >= 0 ? Math.round(raw) : -Math.round(-raw);
}

export function stalledDays(stageEnteredAt: string | null | undefined): number {
  if (!stageEnteredAt) return 0;
  const ms = Date.now() - new Date(stageEnteredAt).getTime();
  return Math.max(0, Math.floor(ms / (24 * 60 * 60 * 1000)));
}

export function DealCard({
  deal,
  onClick,
  dragHandleProps,
  dragging = false,
}: {
  deal: Deal;
  onClick: () => void;
  dragHandleProps?: Record<string, unknown>;
  dragging?: boolean;
}) {
  const age = stalledDays(deal.stage_entered_at);
  return (
    <div
      data-testid={`deal-card-${deal.id}`}
      onClick={onClick}
      className={`rounded-lg border bg-gf-card p-gf-md cursor-pointer transition-shadow ${
        deal.stalled
          ? "border-l-4 border-l-gf-status-warning border-gf-subtle"
          : "border-gf-subtle"
      } ${dragging ? "opacity-60 shadow-lg" : "hover:shadow-sm"}`}
      {...dragHandleProps}
    >
      <p className="text-gf-body font-medium text-gf-primary">{deal.name}</p>
      <p className="text-gf-caption text-gf-secondary">
        {formatMoney(deal.amount_minor, deal.currency)} · {age}d
      </p>
      <div className="flex gap-gf-xs mt-gf-xs">
        {deal.stalled && (
          <span
            title={`No activity for ${age} days`}
            className="text-gf-caption text-gf-status-warning font-medium"
          >
            Stalled {age}d
          </span>
        )}
        {deal.stakeholder_count === 1 && (
          <span
            title="Only one stakeholder captured on this deal"
            className="text-gf-caption text-gf-status-danger font-medium"
          >
            Single-threaded
          </span>
        )}
      </div>
    </div>
  );
}

export function DraggableDealCard({
  deal,
  onClick,
}: {
  deal: Deal;
  onClick: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: deal.id,
  });
  const style = transform
    ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)` }
    : undefined;
  return (
    <div ref={setNodeRef} style={style} {...listeners} {...attributes}>
      <DealCard deal={deal} onClick={onClick} dragging={isDragging} />
    </div>
  );
}
