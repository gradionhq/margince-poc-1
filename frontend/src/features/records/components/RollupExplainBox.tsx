import { useState } from "react";
import type { OrganizationHierarchyRollup } from "../api/records.js";
import { formatMoneyDeDE } from "./RollupTilesBand.js";

export function RollupExplainBox({
  rollup,
  selfFigure,
  childrenSumFigure,
}: {
  rollup: OrganizationHierarchyRollup;
  selfFigure: { amount_minor: number; currency: string };
  childrenSumFigure: { amount_minor: number; currency: string };
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="p-gf-md">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="text-gf-caption text-gf-accent underline"
      >
        Explain this roll-up
      </button>
      {open && (
        <div
          data-testid="rollup-explain-box-content"
          className="mt-gf-sm rounded-md border border-gf-subtle bg-gf-card p-gf-md text-gf-body text-gf-primary"
        >
          <p className="font-mono text-gf-caption">
            roll-up(node) = self(node) + Σ roll-up(child)
          </p>
          <div className="mt-gf-sm flex gap-gf-lg">
            <div>
              <p className="text-gf-caption text-gf-secondary">Self</p>
              <p className="font-semibold">
                {formatMoneyDeDE(selfFigure.amount_minor, selfFigure.currency)}
              </p>
            </div>
            <div>
              <p className="text-gf-caption text-gf-secondary">Children sum</p>
              <p className="font-semibold">
                {formatMoneyDeDE(
                  childrenSumFigure.amount_minor,
                  childrenSumFigure.currency,
                )}
              </p>
            </div>
          </div>
          {rollup.restricted_excluded.length > 0 && (
            <div className="mt-gf-sm">
              <p className="text-gf-caption text-gf-secondary">
                Excluded (restricted — not summed):
              </p>
              <ul className="mt-gf-xs list-disc pl-gf-md text-gf-caption text-gf-primary">
                {rollup.restricted_excluded.map((entry) => (
                  <li key={entry.id}>{entry.display_name}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
