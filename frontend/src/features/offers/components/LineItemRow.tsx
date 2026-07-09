import { useMemo, useState } from "react";
import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { Button } from "../../../shared/ui/forge.js";
import { computeLineNet } from "../lib/offerMath.js";

function formatMinor(value: number) {
  return new Intl.NumberFormat("en-US").format(value);
}

function toNumber(value: string) {
  if (value.trim() === "") return 0;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function LineItemRow({
  line,
  canMutateOffer,
  onUpdate,
  onDelete,
}: {
  line: OfferLineItem;
  canMutateOffer: boolean;
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
  const [quantity, setQuantity] = useState(String(line.quantity));
  const [unitPriceMinor, setUnitPriceMinor] = useState(
    String(line.unit_price_minor),
  );
  const [discountPct, setDiscountPct] = useState(String(line.discount_pct));
  const [taxRate, setTaxRate] = useState(String(line.tax_rate));

  const netMinor = useMemo(
    () =>
      computeLineNet(
        toNumber(quantity),
        toNumber(unitPriceMinor),
        toNumber(discountPct),
      ),
    [discountPct, quantity, unitPriceMinor],
  );

  const commit = () => {
    onUpdate(line.id, {
      quantity: toNumber(quantity),
      unit_price_minor: toNumber(unitPriceMinor),
      discount_pct: toNumber(discountPct),
      tax_rate: toNumber(taxRate),
    });
  };

  return (
    <tr>
      <td className="py-gf-xs pr-gf-sm">{line.description}</td>
      <td className="py-gf-xs pr-gf-sm">{line.unit}</td>
      <td className="py-gf-xs pr-gf-sm">
        {canMutateOffer ? (
          <input
            aria-label={`qty ${line.description}`}
            type="number"
            step="any"
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            onBlur={commit}
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          />
        ) : (
          line.quantity
        )}
      </td>
      <td className="py-gf-xs pr-gf-sm">
        {canMutateOffer ? (
          <input
            aria-label={`unit price ${line.description}`}
            type="number"
            step="any"
            value={unitPriceMinor}
            onChange={(e) => setUnitPriceMinor(e.target.value)}
            onBlur={commit}
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          />
        ) : (
          formatMinor(line.unit_price_minor)
        )}
      </td>
      <td className="py-gf-xs pr-gf-sm">
        {canMutateOffer ? (
          <input
            aria-label={`discount ${line.description}`}
            type="number"
            step="any"
            value={discountPct}
            onChange={(e) => setDiscountPct(e.target.value)}
            onBlur={commit}
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          />
        ) : (
          line.discount_pct
        )}
      </td>
      <td className="py-gf-xs pr-gf-sm">
        {canMutateOffer ? (
          <input
            aria-label={`tax rate ${line.description}`}
            type="number"
            step="any"
            value={taxRate}
            onChange={(e) => setTaxRate(e.target.value)}
            onBlur={commit}
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          />
        ) : (
          line.tax_rate
        )}
      </td>
      <td className="py-gf-xs pr-gf-sm">{formatMinor(netMinor)}</td>
      <td className="py-gf-xs pr-gf-sm">
        {canMutateOffer ? (
          <Button type="button" variant="secondary" onClick={() => onDelete(line.id)}>
            Delete
          </Button>
        ) : null}
      </td>
    </tr>
  );
}
