import { useState } from "react";
import type { QuotaAttainment } from "../api/quotas.js";
import { formatMoneyDeDE } from "./RollupTilesBand.js";

export function QuotaExplainBox({
  attainment,
}: {
  attainment: QuotaAttainment;
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="px-gf-lg pb-gf-lg">
      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          className="text-gf-caption text-gf-accent underline"
        >
          Explain this number
        </button>
        <span className="inline-flex items-center gap-gf-xs rounded-full bg-gf-status-info-subtle px-gf-sm py-gf-xs font-mono text-gf-micro text-gf-status-info">
          computed server-side
        </span>
      </div>
      {open && (
        <div
          data-testid="quota-explain-box-content"
          className="mt-gf-sm rounded-md border border-gf-subtle bg-gf-card p-gf-md font-mono text-gf-caption text-gf-secondary"
        >
          <p>
            attainment = Σ(closed-won base_value) ÷ target, calculated to the
            cent
          </p>
          <p className="mt-gf-xs">
            closed-won ={" "}
            {attainment.contributing_deals
              .map((d) =>
                formatMoneyDeDE(d.base_value_minor, attainment.currency),
              )
              .join(" + ")}
          </p>
          <p className="mt-gf-xs">
            ={" "}
            <b className="text-gf-primary">
              {formatMoneyDeDE(
                attainment.closed_won_minor,
                attainment.currency,
              )}
            </b>{" "}
            ({attainment.contributing_deals.length} deals, close_date in this
            period)
          </p>
          <p className="mt-gf-xs">
            target ={" "}
            <b className="text-gf-primary">
              {formatMoneyDeDE(attainment.target_minor, attainment.currency)}
            </b>{" "}
            <span className="text-gf-status-success">(human-set)</span>
          </p>
          <p className="mt-gf-xs">
            attainment ={" "}
            {formatMoneyDeDE(attainment.closed_won_minor, attainment.currency)}{" "}
            ÷ {formatMoneyDeDE(attainment.target_minor, attainment.currency)} ={" "}
            <b className="text-gf-primary">
              {Math.round(attainment.attainment_pct)}%
            </b>
          </p>
          <p className="mt-gf-xs text-gf-tertiary">
            open / lost / omitted deals are excluded; clean-core only
          </p>
        </div>
      )}
    </div>
  );
}
