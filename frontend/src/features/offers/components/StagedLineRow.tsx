import { useMemo, useState } from "react";
import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { Button, Chip } from "../../../shared/ui/forge.js";
import { LineProvenanceBadge } from "./LineProvenanceBadge.js";

function toNumber(value: string) {
  if (value.trim() === "") return 0;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function isPriced(line: OfferLineItem) {
  return line.price_grounded && line.unit_price_minor > 0;
}

export function StagedLineRow({
  line,
  canMutateOffer,
  currentUserId,
  onAccept,
  onDismiss,
}: {
  line: OfferLineItem;
  canMutateOffer: boolean;
  currentUserId: string;
  onAccept: (
    lineId: string,
    patch: Record<string, string | number>,
  ) => Promise<void> | void;
  onDismiss: (lineId: string) => Promise<void> | void;
}) {
  const [editing, setEditing] = useState(false);
  const [quantity, setQuantity] = useState(String(line.quantity));
  const [unitPriceMinor, setUnitPriceMinor] = useState(String(line.unit_price_minor));
  const [discountPct, setDiscountPct] = useState(String(line.discount_pct));
  const [taxRate, setTaxRate] = useState(String(line.tax_rate));
  const [message, setMessage] = useState<string | null>(null);

  const acceptDisabled = toNumber(unitPriceMinor) <= 0;
  const citation = useMemo(
    () => line.evidence?.snippet ?? "AI-proposed line",
    [line.evidence],
  );

  const accept = async (patch: Record<string, string | number> = {}) => {
    await onAccept(line.id, {
      ...patch,
      captured_by: `human:${currentUserId}`,
      source: "ui",
    });
    setMessage("Accepted — now part of your draft.");
  };

  return (
    <tr data-testid={`staged-line-${line.id}`}>
      <td className="py-gf-xs pr-gf-sm align-top">
        <div className="space-y-gf-xs">
          <div className="font-medium text-gf-primary">{line.description}</div>
          <LineProvenanceBadge
            source={line.source}
            capturedBy={line.captured_by}
            evidence={line.evidence}
          />
          <blockquote className="rounded-gf-md border border-gf-subtle bg-gf-surface p-gf-sm text-gf-caption text-gf-secondary">
            {citation}
          </blockquote>
          {!isPriced(line) ? (
            <Chip className="bg-gf-status-warning/15 text-gf-status-warning border-gf-status-warning/30">
              We won't guess a number for this line.
            </Chip>
          ) : null}
          {message ? (
            <p className="text-gf-caption text-gf-secondary">{message}</p>
          ) : null}
        </div>
      </td>
      <td className="py-gf-xs pr-gf-sm align-top">{line.unit}</td>
      <td className="py-gf-xs pr-gf-sm align-top">
        {editing && canMutateOffer ? (
          <input
            aria-label={`qty ${line.description.toLowerCase()}`}
            type="number"
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          />
        ) : (
          line.quantity
        )}
      </td>
      <td className="py-gf-xs pr-gf-sm align-top">
        {editing && canMutateOffer ? (
          <input
            aria-label={`price for ${line.description.toLowerCase()}`}
            type="number"
            value={unitPriceMinor}
            onChange={(e) => setUnitPriceMinor(e.target.value)}
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          />
        ) : (
          line.unit_price_minor
        )}
      </td>
      <td className="py-gf-xs pr-gf-sm align-top">
        {editing && canMutateOffer ? (
          <input
            aria-label={`discount ${line.description.toLowerCase()}`}
            type="number"
            value={discountPct}
            onChange={(e) => setDiscountPct(e.target.value)}
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          />
        ) : (
          line.discount_pct
        )}
      </td>
      <td className="py-gf-xs pr-gf-sm align-top">
        {editing && canMutateOffer ? (
          <input
            aria-label={`tax rate ${line.description.toLowerCase()}`}
            type="number"
            value={taxRate}
            onChange={(e) => setTaxRate(e.target.value)}
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          />
        ) : (
          line.tax_rate
        )}
      </td>
      <td className="py-gf-xs pr-gf-sm align-top">
        {canMutateOffer ? (
          <div className="flex flex-wrap gap-gf-xs">
            {editing ? (
              <Button
                type="button"
                onClick={async () => {
                  await accept({
                    quantity: toNumber(quantity),
                    unit_price_minor: toNumber(unitPriceMinor),
                    discount_pct: toNumber(discountPct),
                    tax_rate: toNumber(taxRate),
                  });
                  setEditing(false);
                }}
              >
                Save edits
              </Button>
            ) : (
              <Button type="button" onClick={() => setEditing(true)}>
                Edit
              </Button>
            )}
            <Button
              type="button"
              disabled={acceptDisabled}
              onClick={async () => {
                await accept();
              }}
            >
              Accept
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={async () => {
                await onDismiss(line.id);
                setMessage("Dismissed — removed from this draft.");
              }}
            >
              Dismiss
            </Button>
          </div>
        ) : null}
      </td>
    </tr>
  );
}
