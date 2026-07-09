import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { AiDisclosureBanner } from "./AiDisclosureBanner.js";
import { StagedLineRow } from "./StagedLineRow.js";

export function StagedLinesPanel({
  lines,
  stagedLineIds,
  canMutateOffer,
  currentUserId,
  onAccept,
  onDismiss,
}: {
  lines: OfferLineItem[];
  stagedLineIds: Set<string>;
  canMutateOffer: boolean;
  currentUserId: string;
  onAccept: (
    lineId: string,
    patch: Record<string, string | number>,
  ) => Promise<void> | void;
  onDismiss: (lineId: string) => Promise<void> | void;
}) {
  const staged = lines.filter((line) => stagedLineIds.has(line.id));

  if (staged.length === 0) return null;

  return (
    <section
      data-testid="staged-lines-panel"
      className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg"
    >
      <h3 className="text-gf-body font-medium text-gf-primary">
        Staged AI lines
      </h3>
      <AiDisclosureBanner hasEvidenceLines aiDisclosureText={null} />
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
              <th className="py-gf-xs pr-gf-sm">Actions</th>
            </tr>
          </thead>
          <tbody>
            {staged.map((line) => (
              <StagedLineRow
                key={line.id}
                line={line}
                canMutateOffer={canMutateOffer}
                currentUserId={currentUserId}
                onAccept={onAccept}
                onDismiss={onDismiss}
              />
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
