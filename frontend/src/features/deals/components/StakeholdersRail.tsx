import type { Relationship } from "../../../lib/api-client/generated/index.js";
import { StatusBadge } from "../../../shared/ui/forge.js";
import { usePerson } from "../../people/api/people.js";

export function roleBadge(role: string | null | undefined): {
  label: string;
  variant: "accent" | "info" | "error";
} {
  if (role === "champion") return { label: "Champion", variant: "accent" };
  if (role === "blocker") return { label: "Blocker", variant: "error" };
  // economic_buyer, influencer, user, and any other/unknown role render as "Stakeholder".
  return { label: "Stakeholder", variant: "info" };
}

function StakeholderRow({ relationship }: { relationship: Relationship }) {
  const { data: person, isLoading } = usePerson(
    relationship.person_id ?? undefined,
  );
  const badge = roleBadge(relationship.role);
  return (
    <li
      data-testid={`stakeholder-row-${relationship.id}`}
      className="flex items-center justify-between gap-gf-sm p-gf-sm border-b border-gf-subtle last:border-b-0"
    >
      <div>
        {isLoading ? (
          <p className="text-gf-body text-gf-secondary">Loading…</p>
        ) : (
          <>
            <p className="text-gf-body font-medium text-gf-primary">
              {person?.full_name ?? "Unknown"}
            </p>
            {person?.title && (
              <p className="text-gf-caption text-gf-secondary">
                {person.title}
              </p>
            )}
          </>
        )}
      </div>
      <StatusBadge label={badge.label} variant={badge.variant} />
    </li>
  );
}

export function StakeholdersRail({
  stakeholders,
  stakeholderCount,
}: {
  stakeholders: Relationship[];
  stakeholderCount: number | undefined;
}) {
  const hasEconomicBuyer = stakeholders.some(
    (s) => s.role === "economic_buyer",
  );
  const count = stakeholderCount ?? stakeholders.length;

  return (
    <div
      data-testid="stakeholders-rail"
      className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md"
    >
      <h3 className="text-gf-body font-semibold text-gf-primary mb-gf-sm">
        Stakeholders
      </h3>
      {/* count === 0 renders neither framing — the empty state below is the only honest
          statement for zero stakeholders; "single-threaded" would be false (that means exactly
          one), not just under-threaded. */}
      {count > 1 && (
        <p className="text-gf-caption text-gf-status-success mb-gf-sm">
          Multi-threaded
        </p>
      )}
      {count === 1 && (
        <p className="text-gf-caption text-gf-status-danger mb-gf-sm">
          Single-threaded — only one stakeholder captured on this deal
        </p>
      )}
      {!hasEconomicBuyer && (
        <p className="text-gf-caption text-gf-status-warning mb-gf-sm">
          No economic buyer identified yet
        </p>
      )}
      {stakeholders.length === 0 ? (
        <p className="text-gf-body text-gf-secondary">
          No stakeholders captured yet.
        </p>
      ) : (
        <ul>
          {stakeholders.map((s) => (
            <StakeholderRow key={s.id} relationship={s} />
          ))}
        </ul>
      )}
    </div>
  );
}
