import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { Button, Divider } from "../../../shared/ui/forge.js";
import { computeOfferTotals } from "../lib/offerMath.js";
import { LineItemRow } from "./LineItemRow.js";

function isStaged(line: OfferLineItem) {
  return line.evidence != null && line.captured_by.startsWith("agent:");
}

export function LineItemEditor({
  lines,
  canMutateOffer,
  onCreate,
  onUpdate,
  onDelete,
}: {
  lines: OfferLineItem[];
  canMutateOffer: boolean;
  onCreate: () => void;
  onUpdate: (
    lineId: string,
    patch: {
      quantity: number;
      unit_price_minor: number;
      discount_pct: number;
      tax_rate: number;
    },
  ) => void;
  onDelete: (lineId: string) => void;
}) {
  const visibleLines = lines.filter((line) => !isStaged(line));
  const stagedCount = lines.length - visibleLines.length;

  if (lines.length === 0) {
    return (
      <div className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg">
        <h3 className="text-gf-body font-medium text-gf-primary">
          No line items yet
        </h3>
        <p className="mt-gf-xs text-gf-caption text-gf-secondary">
          Add the first line to start building this offer.
        </p>
      </div>
    );
  }

  const totals = computeOfferTotals(
    visibleLines.map((line) => ({
      quantity: line.quantity,
      unitPriceMinor: line.unit_price_minor,
      discountPct: line.discount_pct,
      taxRate: line.tax_rate,
    })),
  );

  return (
    <section>
      <div className="overflow-x-auto">
        <table className="min-w-full text-left text-gf-body">
          <thead>
            <tr className="text-gf-secondary">
              <th className="py-gf-xs pr-gf-sm">Description</th>
              <th className="py-gf-xs pr-gf-sm">Unit</th>
              <th className="py-gf-xs pr-gf-sm">Qty</th>
              <th className="py-gf-xs pr-gf-sm">Unit price</th>
              <th className="py-gf-xs pr-gf-sm">Discount%</th>
              <th className="py-gf-xs pr-gf-sm">Tax rate</th>
              <th className="py-gf-xs pr-gf-sm">Net</th>
              <th className="py-gf-xs pr-gf-sm" />
            </tr>
          </thead>
          <tbody>
            {visibleLines.map((line) => (
              <LineItemRow
                key={line.id}
                line={line}
                canMutateOffer={canMutateOffer}
                onUpdate={onUpdate}
                onDelete={onDelete}
              />
            ))}
          </tbody>
        </table>
      </div>

      {canMutateOffer ? (
        <div className="mt-gf-sm">
          <Button type="button" onClick={onCreate}>
            Add line
          </Button>
        </div>
      ) : null}

      <Divider className="my-gf-md" />

      <div className="text-gf-caption text-gf-secondary">
        <p>Net minor units: {totals.netMinor}</p>
        <p>Tax minor units: {totals.taxMinor}</p>
        <p>Gross minor units: {totals.grossMinor}</p>
        {stagedCount > 0 ? (
          <p>Excludes {stagedCount} staged AI-proposed line(s) from this total.</p>
        ) : null}
      </div>
    </section>
  );
}
