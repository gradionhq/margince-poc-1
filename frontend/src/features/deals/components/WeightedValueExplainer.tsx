import { useRef, useState } from "react";
import { PopoverPortal } from "../../../shared/ui/forge.js";
import { formatMoney, weightedValue } from "./DealCard.js";

export function WeightedValueExplainer({
  amountMinor,
  currency,
  winProbability,
  stageName,
}: {
  amountMinor: number | null | undefined;
  currency: string | null | undefined;
  winProbability: number;
  stageName: string;
}) {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement>(null);
  const weighted = weightedValue(amountMinor, winProbability);

  return (
    <div>
      <p className="text-gf-caption text-gf-secondary">
        {formatMoney(amountMinor, currency)}
      </p>
      <p className="text-gf-body font-semibold text-gf-primary">
        {formatMoney(weighted, currency)} weighted
      </p>
      <p className="text-gf-caption text-gf-muted">
        stage default (deterministic) · {stageName}
      </p>
      <button
        ref={anchorRef}
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="text-gf-caption text-gf-accent underline"
      >
        Explain this number
      </button>
      {open && (
        <PopoverPortal
          anchorRef={anchorRef}
          placement="bottom-left"
          onClickOutside={() => setOpen(false)}
        >
          <div
            data-testid="weighted-value-explainer-popover"
            className="rounded-md border border-gf-subtle bg-gf-card p-gf-md text-gf-body text-gf-primary shadow-lg max-w-xs"
          >
            <p>
              {formatMoney(amountMinor, currency)} × {winProbability}% ={" "}
              {formatMoney(weighted, currency)}
            </p>
            <p className="text-gf-caption text-gf-secondary mt-gf-xs">
              Won = 100%, Lost = 0%
            </p>
          </div>
        </PopoverPortal>
      )}
    </div>
  );
}
