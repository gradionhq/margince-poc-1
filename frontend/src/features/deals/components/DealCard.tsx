import { useDraggable } from "@dnd-kit/core";
import type { Deal } from "../../../lib/api-client/generated/index.js";
import { ContextMenu } from "../../../shared/ui/ContextMenu.js";
import { IconButton } from "../../../shared/ui/forge.js";

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

// Mirrors backend IsStalled's base-timestamp rule (stalled.go): base = last_activity_at
// if set, else created_at. Used for the "Stalled Nd" flag's day-count and hover title —
// deliberately independent of stage_entered_at, since that's what the flag is about.
export function idleDays(
  lastActivityAt: string | null | undefined,
  createdAt: string | null | undefined,
): number {
  const base = lastActivityAt ?? createdAt;
  if (!base) return 0;
  const ms = Date.now() - new Date(base).getTime();
  return Math.max(0, Math.floor(ms / (24 * 60 * 60 * 1000)));
}

export function DealCard({
  deal,
  onClick,
  onAdvanceClick,
  onArchive,
  dragHandleProps,
  dragging = false,
}: {
  deal: Deal;
  onClick: () => void;
  onAdvanceClick?: (dealId: string) => void;
  onArchive?: (dealId: string) => void;
  dragHandleProps?: Record<string, unknown>;
  dragging?: boolean;
}) {
  const age = stalledDays(deal.stage_entered_at);
  const idle = idleDays(deal.last_activity_at, deal.created_at);
  return (
    // biome-ignore lint/a11y/useSemanticElements: can't be a real <button> — it nests the Advance <button> and hosts dnd-kit's drag-handle attributes; role="button" + onKeyDown keeps it operable.
    <div
      data-testid={`deal-card-${deal.id}`}
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") onClick();
      }}
      className={`relative rounded-lg border bg-gf-card p-gf-md cursor-pointer transition-shadow ${
        deal.stalled
          ? "border-l-4 border-l-gf-status-warning border-gf-subtle"
          : "border-gf-subtle"
      } ${dragging ? "opacity-60 shadow-lg" : "hover:shadow-sm"}`}
      {...dragHandleProps}
    >
      {onArchive && (
        <div className="absolute right-gf-xs top-gf-xs">
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
                onSelect: () => onArchive(deal.id),
              },
            ]}
          />
        </div>
      )}
      <p className="text-gf-body font-medium text-gf-primary">{deal.name}</p>
      <p className="text-gf-caption text-gf-secondary">
        {formatMoney(deal.amount_minor, deal.currency)} · {age}d
      </p>
      <div className="flex gap-gf-xs mt-gf-xs">
        {deal.stalled && (
          <span
            title={`No activity for ${idle} days`}
            className="text-gf-caption text-gf-status-warning font-medium"
          >
            Stalled {idle}d
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
      {onAdvanceClick && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation(); // don't also fire the card's own onClick/navigate
            onAdvanceClick(deal.id);
          }}
          className="mt-gf-xs text-gf-caption text-gf-accent underline"
        >
          Advance
        </button>
      )}
    </div>
  );
}

export function DraggableDealCard({
  deal,
  onClick,
  onAdvanceClick,
  onArchive,
}: {
  deal: Deal;
  onClick: () => void;
  onAdvanceClick?: (dealId: string) => void;
  onArchive?: (dealId: string) => void;
}) {
  const { attributes, listeners, setNodeRef, transform, isDragging } =
    useDraggable({
      id: deal.id,
    });
  const style = transform
    ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)` }
    : undefined;
  return (
    <div ref={setNodeRef} style={style} {...listeners} {...attributes}>
      <DealCard
        deal={deal}
        onClick={onClick}
        onAdvanceClick={onAdvanceClick}
        onArchive={onArchive}
        dragging={isDragging}
      />
    </div>
  );
}
