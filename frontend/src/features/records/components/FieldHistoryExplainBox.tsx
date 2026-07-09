import { useState } from "react";
import { computeNetTaxGross } from "../api/fieldHistory.js";
import { formatMoneyDeDE } from "./RollupTilesBand.js";

export function FieldHistoryExplainBox({
  grossMinor,
  currency,
}: {
  grossMinor: number;
  currency: string;
}) {
  const [open, setOpen] = useState(false);
  const { netMinor, taxMinor } = computeNetTaxGross(grossMinor);

  return (
    <div className="mt-gf-xs">
      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          className="text-gf-caption text-gf-accent underline"
        >
          Explain this number
        </button>
        <span className="inline-flex items-center gap-gf-xs rounded-full bg-gf-status-info-subtle px-gf-sm py-gf-xs font-mono text-gf-micro text-gf-status-info">
          computed server-side · never free-typed
        </span>
      </div>
      {open && (
        <div
          data-testid="field-history-explain-box-content"
          className="mt-gf-sm rounded-md border border-gf-subtle bg-gf-card p-gf-md font-mono text-gf-caption text-gf-secondary"
        >
          <p>
            net{" "}
            <b className="text-gf-primary">
              {formatMoneyDeDE(netMinor, currency)}
            </b>{" "}
            + 19% MwSt.{" "}
            <b className="text-gf-primary">
              {formatMoneyDeDE(taxMinor, currency)}
            </b>{" "}
            ={" "}
            <b className="text-gf-primary">
              {formatMoneyDeDE(grossMinor, currency)}
            </b>
          </p>
          <p className="mt-gf-xs text-gf-tertiary">
            money is computed to the cent, in integer minor units — never
            free-typed
          </p>
        </div>
      )}
    </div>
  );
}
