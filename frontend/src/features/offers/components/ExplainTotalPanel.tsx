import { useState } from "react";
import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { Button } from "../../../shared/ui/forge.js";
import { computeLineNet, computeLineTax, computeOfferTotals } from "../lib/offerMath.js";

function isStaged(line: OfferLineItem) {
  return line.evidence != null && line.captured_by.startsWith("agent:");
}

function isPriced(line: OfferLineItem) {
  return !isStaged(line) && line.price_grounded && line.unit_price_minor > 0;
}

function formatFormula(line: OfferLineItem) {
  const net = computeLineNet(line.quantity, line.unit_price_minor, line.discount_pct);
  const tax = computeLineTax(net, line.tax_rate);
  return {
    net,
    tax,
    text: `${line.quantity} × ${line.unit_price_minor} × (1 - ${line.discount_pct}%) = ${net}; ${net} × ${line.tax_rate}% = ${tax}`,
  };
}

function formatGrossCaption(stagedCount: number) {
  if (stagedCount === 0) {
    return "Gross minor units are computed server-side.";
  }
  return `The persisted record still carries ${stagedCount} pending AI line(s) not yet reflected in this figure.`;
}

export function ExplainTotalPanel({
  currency,
  lines,
  grossMinor,
}: {
  currency: string;
  lines: OfferLineItem[];
  grossMinor: number;
}) {
  const [open, setOpen] = useState(false);
  const visibleLines = lines.filter(isPriced);
  const stagedCount = lines.filter(isStaged).length;
  const unpricedCount = lines.filter((line) => !isStaged(line) && (!line.price_grounded || line.unit_price_minor <= 0)).length;
  const totals = computeOfferTotals(
    visibleLines.map((line) => ({
      quantity: line.quantity,
      unitPriceMinor: line.unit_price_minor,
      discountPct: line.discount_pct,
      taxRate: line.tax_rate,
    })),
  );

  return (
    <section className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg">
      <Button type="button" variant="secondary" onClick={() => setOpen((current) => !current)}>
        Explain this total
      </Button>

      {open ? (
        <div className="mt-gf-md space-y-gf-md text-gf-body">
          <p className="text-gf-caption text-gf-secondary">
            amounts in {currency} minor units (cents); ISO 4217
          </p>

          <div className="space-y-gf-sm">
            {visibleLines.map((line) => {
              const formula = formatFormula(line);
              return (
                <div key={line.id} className="rounded-gf-md border border-gf-subtle bg-gf-surface p-gf-sm">
                  <p className="font-medium text-gf-primary">{line.description}</p>
                  <p className="text-gf-caption text-gf-secondary">{formula.text}</p>
                </div>
              );
            })}
          </div>

          <p className="text-gf-caption text-gf-secondary">
            {stagedCount} staged AI-proposed line(s) and {unpricedCount} unpriced line(s) are excluded from this total until accepted/priced.
          </p>

          <div className="text-gf-caption text-gf-secondary">
            <p>Net minor units: {totals.netMinor}</p>
            <p>Tax minor units: {totals.taxMinor}</p>
            <p>Gross minor units: {totals.grossMinor}</p>
            <p>Persisted record total: {grossMinor}</p>
            <p>{formatGrossCaption(stagedCount)}</p>
          </div>
        </div>
      ) : null}
    </section>
  );
}
