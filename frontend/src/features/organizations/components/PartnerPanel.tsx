import type { Deal, Partner } from "../../../lib/api-client/generated/index.js";
import { SectionHeader } from "../../../shared/ui/forge.js";

export function PartnerPanel({
  partner,
  sourcedDeals,
}: {
  partner: Partner | null | undefined;
  sourcedDeals: Deal[];
}) {
  // getPartner 404 → partner is null (STATE-1: an expected non-partner state) — the whole panel
  // is absent, not a hidden skeleton or an error card. Also absent while still loading
  // (undefined) to avoid a flash of an empty panel before the read resolves.
  if (!partner) return null;

  return (
    <div className="p-gf-lg rounded-lg border border-gf-subtle bg-gf-card">
      <SectionHeader label="Partner" />
      <dl className="mt-gf-sm grid grid-cols-3 gap-gf-sm text-gf-caption">
        <div>
          <dt className="text-gf-secondary">Cert status</dt>
          <dd className="text-gf-primary font-medium">{partner.cert_status}</dd>
        </div>
        <div>
          <dt className="text-gf-secondary">Role</dt>
          <dd className="text-gf-primary font-medium">
            {partner.partner_role}
          </dd>
        </div>
        <div>
          <dt className="text-gf-secondary">Margin tier</dt>
          <dd className="text-gf-primary font-medium">
            {partner.margin_tier ?? "—"}
          </dd>
        </div>
      </dl>
      <p className="mt-gf-md text-gf-caption text-gf-secondary font-medium">
        Deals it sourced
      </p>
      {sourcedDeals.length === 0 ? (
        <p className="text-gf-body text-gf-muted">None yet.</p>
      ) : (
        <ul className="mt-gf-xs flex flex-col gap-gf-xs">
          {sourcedDeals.map((d) => (
            <li key={d.id} className="text-gf-body text-gf-primary">
              {d.name}
            </li>
          ))}
        </ul>
      )}
      {/* Known correctness gap (Global Constraints #3): useSourcedDeals only scans the 200 newest
          deals workspace-wide (no partner_org_id filter exists on listDeals) — surfaced in the UI,
          not just a code comment, so a partner with an older sourced deal sees why it's missing. */}
      <p className="mt-gf-sm text-gf-caption text-gf-muted italic">
        Among the 200 most recent deals workspace-wide — an older sourced deal
        may not appear yet.
      </p>
    </div>
  );
}
