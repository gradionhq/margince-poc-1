import { Link } from "react-router-dom";
import type { Deal } from "../../../lib/api-client/generated/index.js";
import { SectionHeader } from "../../../shared/ui/forge.js";
import { openAndWonDeals } from "../api/orgSelectors.js";

export function formatMoney(
  amountMinor: number | null | undefined,
  currency: string | null | undefined,
): string {
  if (amountMinor == null || !currency) return "—";
  return new Intl.NumberFormat(undefined, { style: "currency", currency }).format(
    amountMinor / 100,
  );
}

export function DealRail({ deals }: { deals: Deal[] }) {
  const visible = openAndWonDeals(deals);
  return (
    <div className="p-gf-lg rounded-lg border border-gf-subtle bg-gf-card">
      <SectionHeader label="Deals" />
      {visible.length === 0 && (
        <p className="mt-gf-sm text-gf-body text-gf-muted">No open or won deals for this org.</p>
      )}
      <ul className="mt-gf-sm flex flex-col gap-gf-sm">
        {visible.map((deal) => (
          <li key={deal.id}>
            <Link
              to={`/deals/${deal.id}`}
              className="flex items-center justify-between p-gf-sm rounded-md hover:bg-gf-hover"
            >
              <span className="text-gf-body font-medium text-gf-primary">{deal.name}</span>
              <span className="text-gf-caption text-gf-secondary">
                {formatMoney(deal.amount_minor, deal.currency)}
              </span>
              <span className="flex gap-gf-sm">
                {deal.stalled && (
                  <span className="text-gf-caption text-gf-status-warning font-medium">
                    Stalled
                  </span>
                )}
                {deal.stakeholder_count === 1 && (
                  <span className="text-gf-caption text-gf-status-danger font-medium">
                    Single-threaded
                  </span>
                )}
                <span className="text-gf-caption text-gf-muted uppercase">{deal.status}</span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
