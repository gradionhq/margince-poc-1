import { Link } from "react-router-dom";
import type { Organization, Person } from "../../../lib/api-client/generated/index.js";
import { SectionHeader } from "../../../shared/ui/forge.js";
import { isChampion } from "../api/orgSelectors.js";
import { OrgLogo } from "./OrgLogo.js";

export function PeopleRail({
  contacts,
  org,
}: {
  contacts: Array<{
    id: string;
    data: Person | undefined;
    isLoading: boolean;
    isError: boolean;
  }>;
  org: Organization;
}) {
  const topScore = Math.max(
    0,
    ...contacts.map((c) => c.data?.strength?.score ?? 0),
  );

  return (
    <div className="p-gf-lg rounded-lg border border-gf-subtle bg-gf-card">
      <SectionHeader label="People" />
      {contacts.length === 0 && (
        <p className="mt-gf-sm text-gf-body text-gf-muted">No known contacts yet.</p>
      )}
      <ul className="mt-gf-sm flex flex-col gap-gf-sm">
        {contacts.map((c) => {
          if (c.isError) {
            return (
              <li key={c.id} className="text-gf-caption text-gf-status-danger">
                Couldn't load this contact.
              </li>
            );
          }
          if (c.isLoading || !c.data) {
            return (
              <li key={c.id} className="text-gf-caption text-gf-muted">
                Loading…
              </li>
            );
          }
          const person = c.data;
          const score = person.strength?.score;
          const champion = isChampion(person.id, org);
          const isTop = score != null && score === topScore && topScore > 0;
          return (
            <li key={c.id}>
              <Link
                to={`/people/${person.id}`}
                className={`flex items-center gap-gf-sm p-gf-sm rounded-md hover:bg-gf-hover ${
                  isTop ? "ring-1 ring-gf-accent" : ""
                }`}
              >
                <OrgLogo name={person.full_name} size="sm" />
                <div className="flex-1">
                  <p className="text-gf-body font-medium text-gf-primary">
                    {person.full_name}
                  </p>
                  {person.title && (
                    <p className="text-gf-caption text-gf-secondary">{person.title}</p>
                  )}
                </div>
                {champion ? (
                  <span className="text-gf-caption text-gf-accent font-medium">Champion</span>
                ) : (
                  <span className="text-gf-caption text-gf-muted">Stakeholder</span>
                )}
                <span className="text-gf-caption text-gf-secondary">
                  {score != null ? `${score}/100` : "no signal yet"}
                </span>
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
