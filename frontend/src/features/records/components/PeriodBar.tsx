import type { Quota } from "../api/quotas.js";

// Quarter label ("Q3 2026") offset by whole quarters from a YYYY-MM-DD date.
function quarterLabel(dateStr: string, offsetQuarters: number): string {
  const d = new Date(`${dateStr}T00:00:00Z`);
  const totalMonths = d.getUTCFullYear() * 12 + d.getUTCMonth() + offsetQuarters * 3;
  const year = Math.floor(totalMonths / 12);
  const quarter = Math.floor((totalMonths % 12) / 3) + 1;
  return `Q${quarter} ${year}`;
}

export function PeriodBar({
  quota,
  onToast,
}: {
  quota: Quota;
  onToast: (message: string) => void;
}) {
  const current = quarterLabel(quota.period_start, 0);
  const prior = quarterLabel(quota.period_start, -1);
  const next = quarterLabel(quota.period_start, 1);

  return (
    <div className="mt-gf-md flex flex-wrap items-center gap-gf-sm">
      <span className="text-gf-micro font-semibold uppercase tracking-wide text-gf-tertiary">
        Period
      </span>
      <button
        type="button"
        onClick={() => onToast(`${prior} is closed — read-only`)}
        className="rounded-full border border-gf-subtle bg-gf-elevated px-gf-md py-gf-xs font-mono text-gf-caption text-gf-secondary transition-colors hover:border-gf-strong hover:text-gf-primary"
      >
        {prior} · closed
      </button>
      <span className="rounded-full border border-gf-accent bg-gf-accent-light px-gf-md py-gf-xs font-mono text-gf-caption text-gf-accent">
        {current} · current
      </span>
      <button
        type="button"
        onClick={() => onToast(`${next} quota not yet set`)}
        className="rounded-full border border-gf-subtle bg-gf-elevated px-gf-md py-gf-xs font-mono text-gf-caption text-gf-secondary transition-colors hover:border-gf-strong hover:text-gf-primary"
      >
        {next} · not set
      </button>
    </div>
  );
}
