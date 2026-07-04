import { useState } from "react";
import { useParams } from "react-router-dom";
import type { Person } from "../../../lib/api-client/generated/index.js";
import { Button, Skeleton, StatusDot } from "../../../shared/ui/forge.js";
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
import { AccountSignalCard } from "../components/AccountSignalCard.js";
import { ActivityCard } from "../components/ActivityCard.js";
import { DealRail } from "../components/DealRail.js";
import { EditOrgModal } from "../components/EditOrgModal.js";
import { NewDealModal } from "../components/NewDealModal.js";
import { OrgLogo } from "../components/OrgLogo.js";
import { OrgStrengthCard } from "../components/OrgStrengthCard.js";
import { PartnerPanel } from "../components/PartnerPanel.js";
import { PeopleRail } from "../components/PeopleRail.js";
import { QuickFactsRail } from "../components/QuickFactsRail.js";

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
  const { data: sourcedDeals } = useSourcedDeals(partner ? id : undefined);

  const [newDealOpen, setNewDealOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editedFields, setEditedFields] = useState<Set<string>>(new Set());
  const contactPeople = contacts
    .map((c) => c.data)
    .filter((p): p is Person => !!p);

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
            {org.industry && (
              <span>
                {org.industry}
                {editedFields.has("industry") && (
                  <span
                    className="ml-gf-xs inline-flex items-center"
                    title="Typed by you this session"
                  >
                    <StatusDot
                      state="success"
                      size="sm"
                      ariaLabel="Typed by you this session"
                    />
                  </span>
                )}
              </span>
            )}
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
            {org.size_band && (
              <span>
                {org.size_band} staff
                {editedFields.has("size_band") && (
                  <span
                    className="ml-gf-xs inline-flex items-center"
                    title="Typed by you this session"
                  >
                    <StatusDot
                      state="success"
                      size="sm"
                      ariaLabel="Typed by you this session"
                    />
                  </span>
                )}
              </span>
            )}
            {location && (
              <span>
                {location}
                {editedFields.has("location") && (
                  <span
                    className="ml-gf-xs inline-flex items-center"
                    title="Typed by you this session"
                  >
                    <StatusDot
                      state="success"
                      size="sm"
                      ariaLabel="Typed by you this session"
                    />
                  </span>
                )}
              </span>
            )}
          </div>
        </div>
        <div className="ml-auto flex gap-gf-sm">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setEditOpen(true)}
          >
            Edit
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={() => setNewDealOpen(true)}
          >
            New deal
          </Button>
          {/* Native button, not the Forge Button atom: Forge's Button doesn't forward a
              `title` attribute, and the disabled-honest tooltip explaining why is required
              here (same pattern as CompaniesPage's rare-path "New" button). */}
          <button
            type="button"
            disabled
            title="Account summaries ship in a later chapter"
            className="h-8 px-gf-md rounded-md text-gf-small text-gf-muted bg-transparent border border-gf-subtle disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Summarize this account
          </button>
        </div>
      </header>
      <main className="p-gf-lg max-w-5xl mx-auto flex flex-col gap-gf-lg">
        <OrgStrengthCard orgStrength={org.org_strength} contacts={contacts} />
        <div className="grid grid-cols-1 md:grid-cols-2 gap-gf-lg">
          <PeopleRail org={org} contacts={contacts} />
          <DealRail deals={org.deals ?? []} />
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-gf-lg">
          <ActivityCard
            activities={org.activities ?? []}
            source={org.source}
            capturedBy={org.captured_by}
          />
          <AccountSignalCard org={org} />
          <QuickFactsRail org={org} />
        </div>
        <PartnerPanel partner={partner} sourcedDeals={sourcedDeals ?? []} />
      </main>
      <NewDealModal
        open={newDealOpen}
        onClose={() => setNewDealOpen(false)}
        org={org}
        contacts={contactPeople}
      />
      <EditOrgModal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        org={org}
        onSaved={(changed) =>
          setEditedFields((prev) => new Set([...prev, ...changed]))
        }
      />
    </div>
  );
}
