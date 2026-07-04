import { useParams } from "react-router-dom";
import { Skeleton } from "../../../shared/ui/forge.js";
import {
  useOrganization,
  useOrgContacts,
  useOrgPartner,
  useSourcedDeals,
} from "../api/organizations.js";
import {
  formatLocation,
  getEmploymentContactIds,
  primaryDomainUrl,
} from "../api/orgSelectors.js";
import { OrgLogo } from "../components/OrgLogo.js";
import { OrgStrengthCard } from "../components/OrgStrengthCard.js";

export function CompanyDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: org, isLoading, isError, refetch } = useOrganization(id);
  const contactIds = org ? getEmploymentContactIds(org) : [];
  const { contacts } = useOrgContacts(contactIds);
  // Partner + sourced deals fetched here so Task 6 can wire PartnerPanel without touching the
  // page's data-loading shape again.
  //
  // Gate note: `Organization.classification` is a real field (crm.d.ts), but per its own doc
  // comment "an org IS a partner iff classification='partner' AND it has a `partner` row"
  // (A41/ADR-0032) — classification alone can't tell us the partner row exists. useOrgPartner
  // already resolves that 404-vs-row distinction (404 -> null, a legitimate non-error state), so
  // gating on `partner` being non-null is the honest, type-safe signal rather than re-deriving it
  // from classification here.
  const { data: partner } = useOrgPartner(id);
  // Not yet rendered (Task 6 wires PartnerPanel) — fetched now so the query is warm.
  useSourcedDeals(partner ? id : undefined);

  if (isLoading) {
    return (
      <div className="p-gf-lg" data-testid="company-detail-skeleton">
        <Skeleton height="120px" />
      </div>
    );
  }
  if (isError || !org) {
    return (
      <div className="p-gf-lg">
        <p className="text-gf-body text-gf-status-danger mb-gf-sm">
          Failed to load this company.
        </p>
        <button
          type="button"
          onClick={() => refetch()}
          className="text-gf-caption text-gf-accent underline"
        >
          Retry
        </button>
      </div>
    );
  }

  const websiteUrl = primaryDomainUrl(org.domains);
  const location = formatLocation(org.address);

  return (
    <div className="min-h-screen bg-gf-page">
      <header className="px-gf-lg py-gf-lg border-b border-gf-subtle bg-gf-card flex items-center gap-gf-md">
        <OrgLogo name={org.display_name} size="lg" />
        <div>
          <h1 className="text-gf-title font-semibold text-gf-primary">
            {org.display_name}
          </h1>
          <div className="flex gap-gf-md text-gf-caption text-gf-secondary flex-wrap">
            {org.industry && <span>{org.industry}</span>}
            {websiteUrl && (
              <a
                href={websiteUrl}
                target="_blank"
                rel="noreferrer"
                className="text-gf-accent underline"
              >
                {websiteUrl.replace("https://", "")}
              </a>
            )}
            {org.size_band && <span>{org.size_band} staff</span>}
            {location && <span>{location}</span>}
          </div>
        </div>
      </header>
      <main className="p-gf-lg max-w-5xl mx-auto flex flex-col gap-gf-lg">
        <OrgStrengthCard orgStrength={org.org_strength} contacts={contacts} />
        {/* TASK-4-INSERT: PeopleRail + DealRail go here */}
        {/* TASK-5-INSERT: ActivityCard + AccountSignalCard + QuickFactsRail go here */}
        {/* TASK-6-INSERT: PartnerPanel + top-bar actions (Edit/New deal/Summarize) go here.
            `partner` above is already wired for it; sourced deals are fetched (unused) above. */}
      </main>
    </div>
  );
}
