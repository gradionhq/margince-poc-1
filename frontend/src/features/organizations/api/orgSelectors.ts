import type {
  Deal,
  Organization,
} from "../../../lib/api-client/generated/index.js";

export function getEmploymentContactIds(org: Organization): string[] {
  const seen = new Set<string>();
  for (const rel of org.relationships ?? []) {
    if (
      rel.kind === "employment" &&
      rel.organization_id === org.id &&
      !rel.archived_at &&
      rel.person_id
    ) {
      seen.add(rel.person_id);
    }
  }
  return [...seen];
}

// PO-N-CHAMPION: a deal_stakeholder relationship with role "champion" matched (by deal_id) to
// any OPEN deal of this org — one match anywhere suffices.
export function isChampion(personId: string, org: Organization): boolean {
  const openDealIds = new Set(
    (org.deals ?? []).filter((d) => d.status === "open").map((d) => d.id),
  );
  return (org.relationships ?? []).some(
    (rel) =>
      rel.kind === "deal_stakeholder" &&
      rel.person_id === personId &&
      rel.role === "champion" &&
      rel.deal_id != null &&
      openDealIds.has(rel.deal_id),
  );
}

export function primaryDomainUrl(
  domains: Organization["domains"] | undefined,
): string | null {
  if (!domains || domains.length === 0) return null;
  const primary = domains.find((d) => d.is_primary) ?? domains[0];
  return `https://${primary.domain}`;
}

export function formatLocation(
  address: Organization["address"] | undefined,
): string | null {
  if (!address) return null;
  const parts = [address.city, address.country].filter((p): p is string => !!p);
  return parts.length > 0 ? parts.join(", ") : null;
}

export function openAndWonDeals(deals: Deal[]): Deal[] {
  return deals.filter((d) => d.status !== "lost");
}

export function formatWonLifetime(deals: Deal[]): string | null {
  const won = deals.filter((d) => d.status === "won" && d.currency);
  if (won.length === 0) return null;
  const byCurrency = new Map<string, number>();
  for (const d of won) {
    const currency = d.currency as string;
    byCurrency.set(
      currency,
      (byCurrency.get(currency) ?? 0) + (d.amount_minor ?? 0),
    );
  }
  return [...byCurrency.entries()]
    .map(([currency, minor]) =>
      new Intl.NumberFormat(undefined, { style: "currency", currency }).format(
        minor / 100,
      ),
    )
    .join(" · ");
}

export function lastTouchAt(org: Organization): string | null {
  const activities = org.activities ?? [];
  if (activities.length === 0) return org.updated_at;
  return activities.reduce((latest, a) =>
    a.occurred_at > latest.occurred_at ? a : latest,
  ).occurred_at;
}

export function stalledDeal(deals: Deal[]): Deal | undefined {
  return deals.find((d) => d.stalled);
}
