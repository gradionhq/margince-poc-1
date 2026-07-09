import { useState } from "react";
import type { Offer, OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { Button } from "../../../shared/ui/forge.js";
import { useRenderOffer } from "../api/offers.js";
import { getOfferCopy } from "../lib/offerCopy.js";
import { formatMoneyForLocale } from "../lib/money.js";
import { computeLineNet } from "../lib/offerMath.js";

export function OfferPreviewPanel({
  dealName,
  offer,
  lines,
  canMutateOffer,
}: {
  dealName: string;
  offer: Offer;
  lines: OfferLineItem[];
  canMutateOffer: boolean;
}) {
  const [locale, setLocale] = useState<"de" | "en">("de");
  const [pdfAssetRef, setPdfAssetRef] = useState(offer.pdf_asset_ref ?? null);
  const renderOffer = useRenderOffer(offer.id);
  const copy = getOfferCopy(locale);

  const validUntil = new Intl.DateTimeFormat(
    locale === "de" ? "de-DE" : "en-US",
    { dateStyle: "short" },
  ).format(new Date(offer.valid_until));

  const generatePdf = async () => {
    const next = await renderOffer.mutateAsync();
    setPdfAssetRef(next.pdf_asset_ref ?? null);
  };

  return (
    <section className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg">
      <div className="flex items-center justify-between gap-gf-md">
        <div>
          <h2 className="text-gf-body font-medium text-gf-primary">
            {copy.title}
          </h2>
          <p className="text-gf-caption text-gf-secondary">{dealName}</p>
        </div>
        <div className="flex gap-gf-xs">
          <Button type="button" variant={locale === "de" ? "primary" : "secondary"} onClick={() => setLocale("de")}>
            DE
          </Button>
          <Button type="button" variant={locale === "en" ? "primary" : "secondary"} onClick={() => setLocale("en")}>
            EN
          </Button>
        </div>
      </div>

      <dl className="mt-gf-md grid gap-gf-sm text-gf-body sm:grid-cols-3">
        <div>
          <dt className="text-gf-caption text-gf-secondary">
            {copy.meta.offerNumber}
          </dt>
          <dd className="text-gf-primary">{offer.offer_number}</dd>
        </div>
        <div>
          <dt className="text-gf-caption text-gf-secondary">{copy.meta.deal}</dt>
          <dd className="text-gf-primary">{dealName}</dd>
        </div>
        <div>
          <dt className="text-gf-caption text-gf-secondary">
            {copy.meta.validUntil}
          </dt>
          <dd className="text-gf-primary">{validUntil}</dd>
        </div>
      </dl>

      <div className="mt-gf-md overflow-x-auto">
        <table className="min-w-full text-left text-gf-body">
          <thead>
            <tr className="text-gf-secondary">
              <th className="py-gf-xs pr-gf-sm">{copy.lineTable.description}</th>
              <th className="py-gf-xs pr-gf-sm">{copy.lineTable.quantity}</th>
              <th className="py-gf-xs pr-gf-sm">{copy.lineTable.unit}</th>
              <th className="py-gf-xs pr-gf-sm">{copy.lineTable.unitPrice}</th>
              <th className="py-gf-xs pr-gf-sm">{copy.lineTable.discount}</th>
              <th className="py-gf-xs pr-gf-sm">{copy.lineTable.taxRate}</th>
              <th className="py-gf-xs pr-gf-sm">{copy.lineTable.net}</th>
            </tr>
          </thead>
          <tbody>
            {lines.map((line) => {
              const netMinor = computeLineNet(
                line.quantity,
                line.unit_price_minor,
                line.discount_pct,
              );

              return (
                <tr key={line.id}>
                  <td className="py-gf-xs pr-gf-sm">{line.description}</td>
                  <td className="py-gf-xs pr-gf-sm">{line.quantity}</td>
                  <td className="py-gf-xs pr-gf-sm">{line.unit}</td>
                  <td className="py-gf-xs pr-gf-sm">
                    {formatMoneyForLocale(line.unit_price_minor, offer.currency, locale)}
                  </td>
                  <td className="py-gf-xs pr-gf-sm">{line.discount_pct}%</td>
                  <td className="py-gf-xs pr-gf-sm">{line.tax_rate}%</td>
                  <td className="py-gf-xs pr-gf-sm">
                    {formatMoneyForLocale(netMinor, offer.currency, locale)}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <dl className="mt-gf-md grid gap-gf-sm text-gf-body sm:grid-cols-3">
        <div>
          <dt className="text-gf-caption text-gf-secondary">{copy.totals.net}</dt>
          <dd>{formatMoneyForLocale(offer.net_minor, offer.currency, locale)}</dd>
        </div>
        <div>
          <dt className="text-gf-caption text-gf-secondary">{copy.totals.tax}</dt>
          <dd>{formatMoneyForLocale(offer.tax_minor, offer.currency, locale)}</dd>
        </div>
        <div>
          <dt className="text-gf-caption text-gf-secondary">{copy.totals.gross}</dt>
          <dd>{formatMoneyForLocale(offer.gross_minor, offer.currency, locale)}</dd>
        </div>
      </dl>

      <p className="mt-gf-md text-gf-caption text-gf-secondary">{copy.legal}</p>

      {canMutateOffer ? (
        <div className="mt-gf-md flex items-center gap-gf-sm">
          <Button
            type="button"
            variant="secondary"
            disabled={renderOffer.isPending}
            onClick={generatePdf}
          >
            {copy.pdfButton}
          </Button>
          {pdfAssetRef ? (
            <a
              href={pdfAssetRef}
              target="_blank"
              rel="noreferrer"
              className="text-gf-caption text-gf-accent underline"
            >
              {copy.viewPdf}
            </a>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
