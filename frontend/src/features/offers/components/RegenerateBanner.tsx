import { useState } from "react";
import { useNavigate } from "react-router-dom";
import type { Offer } from "../../../lib/api-client/generated/index.js";
import { useRegenerateOffer } from "../api/offers.js";
import { AiDisclosureBanner } from "./AiDisclosureBanner.js";
import { LineProvenanceBadge } from "./LineProvenanceBadge.js";

type OfferDiff = NonNullable<NonNullable<Offer["diff_from_previous"]>>;

function diffCounts(diff: OfferDiff) {
  return {
    added: diff.added?.length ?? 0,
    removed: diff.removed?.length ?? 0,
    changed: diff.changed?.length ?? 0,
  };
}

export function RegenerateBanner({
  dealId,
  offer,
  canMutateOffer,
}: {
  dealId: string;
  offer: Pick<
    Offer,
    "id" | "offer_number" | "revision" | "status" | "currency"
  >;
  canMutateOffer: boolean;
}) {
  const navigate = useNavigate();
  const regenerate = useRegenerateOffer(dealId);
  const [result, setResult] = useState<Offer | null>(null);
  const [showDiff, setShowDiff] = useState(false);

  if (!canMutateOffer || offer.status !== "draft") {
    return null;
  }

  const response = result;
  const diff = response?.diff_from_previous ?? null;
  const counts = response?.diff_from_previous
    ? diffCounts(response.diff_from_previous)
    : null;
  const hasEvidenceLines = !!response?.ai_generated && !!response.ai_disclosure;

  return (
    <section className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg">
      <div className="flex flex-wrap items-center justify-between gap-gf-sm">
        <div>
          <h2 className="text-gf-body font-medium text-gf-primary">
            Regenerate
          </h2>
          <p className="text-gf-caption text-gf-secondary">
            {response && counts
              ? `v${offer.revision} → v${response.revision} — ${counts.added} added, ${counts.removed} removed, ${counts.changed} changed`
              : `v${offer.revision} → v${offer.revision + 1}`}
          </p>
        </div>
        <button
          type="button"
          disabled={regenerate.isPending}
          onClick={async () => {
            const next = await regenerate.mutateAsync({ offerId: offer.id });
            setResult(next);
            navigate(`/deals/${dealId}/offers/${next.id}`);
          }}
          className="rounded-full border border-gf-accent px-gf-sm py-gf-xs text-gf-caption text-gf-accent disabled:opacity-50"
        >
          Regenerate
        </button>
      </div>

      {response ? (
        <>
          <AiDisclosureBanner
            hasEvidenceLines={hasEvidenceLines}
            aiDisclosureText={response.ai_disclosure ?? null}
          />
          {diff ? (
            <div className="mt-gf-md">
              <button
                type="button"
                onClick={() => setShowDiff((current) => !current)}
                className="text-gf-caption text-gf-accent underline"
              >
                View full diff
              </button>
              {showDiff ? (
                <div className="mt-gf-sm overflow-x-auto">
                  <table className="min-w-full text-left text-gf-caption">
                    <thead>
                      <tr className="text-gf-secondary">
                        <th className="py-gf-xs pr-gf-sm">Type</th>
                        <th className="py-gf-xs pr-gf-sm">Description</th>
                        <th className="py-gf-xs pr-gf-sm">Qty</th>
                        <th className="py-gf-xs pr-gf-sm">Unit price</th>
                        <th className="py-gf-xs pr-gf-sm">Discount%</th>
                        <th className="py-gf-xs pr-gf-sm">Tax rate</th>
                        <th className="py-gf-xs pr-gf-sm">Provenance</th>
                      </tr>
                    </thead>
                    <tbody>
                      {diff.added?.map((line) => (
                        <tr key={line.id}>
                          <td>Added</td>
                          <td>{line.description}</td>
                          <td>{line.quantity}</td>
                          <td>{line.unit_price_minor}</td>
                          <td>{line.discount_pct}</td>
                          <td>{line.tax_rate}</td>
                          <td>
                            <LineProvenanceBadge
                              source={line.source}
                              capturedBy={line.captured_by}
                              evidence={line.evidence}
                            />
                          </td>
                        </tr>
                      ))}
                      {diff.removed?.map((line) => (
                        <tr key={line.id}>
                          <td>Removed</td>
                          <td>{line.description}</td>
                          <td>{line.quantity}</td>
                          <td>{line.unit_price_minor}</td>
                          <td>{line.discount_pct}</td>
                          <td>{line.tax_rate}</td>
                          <td>
                            <LineProvenanceBadge
                              source={line.source}
                              capturedBy={line.captured_by}
                              evidence={line.evidence}
                            />
                          </td>
                        </tr>
                      ))}
                      {diff.changed?.map(({ before, after }, index) => (
                        <tr key={after?.id ?? index}>
                          <td>Changed</td>
                          <td>
                            {before?.description} → {after?.description}
                          </td>
                          <td>
                            {before?.quantity} → {after?.quantity}
                          </td>
                          <td>
                            {before?.unit_price_minor} →{" "}
                            {after?.unit_price_minor}
                          </td>
                          <td>
                            {before?.discount_pct} → {after?.discount_pct}
                          </td>
                          <td>
                            {before?.tax_rate} → {after?.tax_rate}
                          </td>
                          <td>
                            <LineProvenanceBadge
                              source={after?.source ?? before?.source ?? ""}
                              capturedBy={
                                after?.captured_by ?? before?.captured_by ?? ""
                              }
                              evidence={
                                after?.evidence ?? before?.evidence ?? null
                              }
                            />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : null}
            </div>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
