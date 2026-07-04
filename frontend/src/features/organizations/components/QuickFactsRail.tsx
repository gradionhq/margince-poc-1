import type { Organization } from "../../../lib/api-client/generated/index.js";
import { SectionHeader } from "../../../shared/ui/forge.js";
import { formatWonLifetime, lastTouchAt } from "../api/orgSelectors.js";

export function QuickFactsRail({ org }: { org: Organization }) {
  const wonLifetime = formatWonLifetime(org.deals ?? []);
  const lastTouch = lastTouchAt(org);
  const facts: Array<[string, string]> = [
    ["Owner", org.owner_id ?? "unassigned"],
    ["Open deals", String(org.open_deal_count ?? 0)],
    ["Won lifetime", wonLifetime ?? "none yet"],
    ["People known", String(org.contact_count ?? 0)],
    ["First seen", new Date(org.created_at).toLocaleDateString()],
    ["Last touch", lastTouch ? new Date(lastTouch).toLocaleDateString() : "—"],
  ];
  return (
    <div className="p-gf-lg rounded-lg border border-gf-subtle bg-gf-card">
      <SectionHeader label="Quick facts" />
      <dl className="mt-gf-sm grid grid-cols-2 gap-gf-sm text-gf-caption">
        {facts.map(([label, value]) => (
          <div key={label}>
            <dt className="text-gf-secondary">{label}</dt>
            <dd className="text-gf-primary font-medium">{value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
