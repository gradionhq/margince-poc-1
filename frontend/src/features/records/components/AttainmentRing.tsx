import { Skeleton } from "../../../shared/ui/forge.js";
import type { QuotaAttainment } from "../api/quotas.js";
import { formatMoneyDeDE } from "./RollupTilesBand.js";

const RING_RADIUS = 68;
const RING_CIRCUMFERENCE = 2 * Math.PI * RING_RADIUS; // matches the corpus mockup's ring geometry

function bandColorClass(band: QuotaAttainment["band"]): string {
  if (band === "met") return "text-gf-status-success";
  if (band === "behind") return "text-gf-status-danger";
  return "text-gf-accent";
}

// AC-quota-3: pace measured against pace_pct (% of period elapsed), never a hardcoded number.
function paceLine(a: QuotaAttainment): { text: string; dotClass: string } {
  const pct = Math.round(a.attainment_pct);
  const elapsed = Math.round(a.pace_pct);
  if (a.attainment_pct >= 100) {
    return { text: "Target met", dotClass: "bg-gf-status-success" };
  }
  if (a.attainment_pct >= a.pace_pct) {
    return {
      text: `Ahead of pace — ${pct}% attained vs ${elapsed}% of period elapsed.`,
      dotClass: "bg-gf-status-success",
    };
  }
  return {
    text: `Behind pace — ${pct}% attained vs ${elapsed}% of period elapsed.`,
    dotClass: "bg-gf-status-danger",
  };
}

export function AttainmentRing({
  attainment,
  isLoading,
  isError,
  isForbidden,
  isTargetZero,
}: {
  attainment: QuotaAttainment | undefined;
  isLoading: boolean;
  isError: boolean;
  isForbidden?: boolean;
  isTargetZero?: boolean;
}) {
  if (isLoading) {
    return (
      <div data-testid="attainment-ring-skeleton" className="flex gap-gf-xl p-gf-lg">
        <Skeleton variant="circle" width="160px" height="160px" />
        <div className="flex flex-1 flex-col gap-gf-md">
          <Skeleton height="20px" />
          <Skeleton height="20px" />
          <Skeleton height="20px" />
        </div>
      </div>
    );
  }
  // STATE-4, checked before STATE-1/generic error — a 403 also sets isError=true.
  if (isForbidden) {
    return (
      <div className="p-gf-lg text-gf-body text-gf-status-danger">
        You don't have access to this quota's attainment.
      </div>
    );
  }
  // STATE-1 (422 attainment_target_zero) — an honest "set a target" prompt, distinct from STATE-3.
  if (isTargetZero) {
    return (
      <div className="p-gf-lg text-gf-body text-gf-secondary">
        No target set for this period. Set a target below to start tracking attainment from
        closed-won deals.
      </div>
    );
  }
  if (isError || !attainment) {
    return (
      <div className="p-gf-lg text-gf-body text-gf-status-danger">
        Couldn't recompute attainment.
      </div>
    );
  }

  const pct = Math.round(attainment.attainment_pct);
  const frac = Math.min(attainment.attainment_pct / 100, 1);
  const dashoffset = RING_CIRCUMFERENCE * (1 - frac);
  const pace = paceLine(attainment);

  return (
    <div className="flex flex-wrap items-center gap-gf-xl p-gf-lg">
      <div className="relative h-[160px] w-[160px] flex-none">
        <svg width="160" height="160" viewBox="0 0 160 160" className="-rotate-90">
          <circle
            cx="80"
            cy="80"
            r={RING_RADIUS}
            fill="none"
            stroke="var(--gf-card)"
            strokeWidth="14"
          />
          <circle
            cx="80"
            cy="80"
            r={RING_RADIUS}
            fill="none"
            strokeWidth="14"
            strokeLinecap="round"
            stroke="currentColor"
            className={bandColorClass(attainment.band)}
            strokeDasharray={RING_CIRCUMFERENCE}
            strokeDashoffset={dashoffset}
          />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-gf-xs">
          <span className="font-mono text-gf-display font-medium text-gf-primary">{pct}%</span>
          <span className="text-gf-micro uppercase tracking-wide text-gf-tertiary">
            attained
          </span>
        </div>
      </div>
      <div className="flex min-w-[240px] flex-1 flex-col gap-gf-md">
        <div className="flex items-baseline justify-between gap-gf-md">
          <span className="text-gf-body font-semibold text-gf-primary">
            Closed-won this period
          </span>
          <span className="font-mono text-gf-title font-medium text-gf-primary">
            {formatMoneyDeDE(attainment.closed_won_minor, attainment.currency)}
          </span>
        </div>
        <div className="h-px bg-gf-subtle" />
        <div className="flex items-baseline justify-between gap-gf-md">
          <span className="text-gf-body text-gf-secondary">Target</span>
          <span className="font-mono text-gf-body text-gf-secondary">
            {formatMoneyDeDE(attainment.target_minor, attainment.currency)}
          </span>
        </div>
        <div className="flex items-baseline justify-between gap-gf-md">
          <span className="text-gf-body text-gf-secondary">Gap to target</span>
          <span className="font-mono text-gf-body text-gf-accent">
            {attainment.gap_minor >= 0 ? "+" : ""}
            {formatMoneyDeDE(attainment.gap_minor, attainment.currency)}
          </span>
        </div>
        <div className="flex items-center gap-gf-sm text-gf-caption text-gf-secondary">
          <span className={`h-2 w-2 rounded-full ${pace.dotClass}`} />
          <span>{pace.text}</span>
        </div>
      </div>
    </div>
  );
}
