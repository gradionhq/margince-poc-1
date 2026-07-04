import { useState } from "react";
import { flushSync } from "react-dom";
import { Link } from "react-router-dom";
import type {
  Organization,
  Person,
} from "../../../lib/api-client/generated/index.js";
import { SectionHeader } from "../../../shared/ui/forge.js";

const bucketBarColor: Record<string, string> = {
  strong: "bg-gf-accent",
  moderate: "bg-gf-status-warning",
  weak: "bg-gf-status-danger",
};

export function OrgStrengthCard({
  orgStrength,
  contacts,
}: {
  orgStrength: Organization["org_strength"];
  contacts: Array<{
    id: string;
    data: Person | undefined;
    isLoading: boolean;
    isError: boolean;
  }>;
}) {
  const [expanded, setExpanded] = useState(false);

  if (!orgStrength) {
    return (
      <div className="p-gf-lg rounded-lg border border-gf-subtle bg-gf-card">
        <SectionHeader label="Account relationship strength" />
        <p className="mt-gf-sm text-gf-body text-gf-muted italic">
          No signal yet — no contact at this org has a computed strength.
        </p>
      </div>
    );
  }

  const barColor = bucketBarColor[orgStrength.bucket] ?? "bg-gf-secondary";

  return (
    <div className="p-gf-lg rounded-lg border border-gf-subtle bg-gf-card">
      <SectionHeader label="Account relationship strength" />
      <div className="flex items-center gap-gf-md mt-gf-sm">
        <div className="w-32 h-2 rounded-full bg-gf-subtle overflow-hidden">
          <div
            className={`h-full rounded-full ${barColor}`}
            style={{ width: `${orgStrength.score}%` }}
          />
        </div>
        <span className="text-gf-title font-semibold text-gf-primary">
          {orgStrength.score}/100
        </span>
        <span className="px-gf-sm py-0.5 rounded-full text-gf-caption bg-gf-accent-light text-gf-accent">
          computed · MAX over contacts
        </span>
      </div>
      <p className="mt-gf-sm text-gf-caption text-gf-secondary">
        Strongest contact:{" "}
        <Link
          to={`/people/${orgStrength.top_person_id}`}
          className="text-gf-accent underline"
        >
          {orgStrength.top_person_name}
        </Link>
      </p>
      <p className="mt-gf-xs text-gf-caption text-gf-muted italic">
        This is the max over contacts — not an average, not a black box.
      </p>
      <button
        type="button"
        onClick={() => {
          // flushSync: this toggle must commit synchronously so a plain DOM
          // click (not just React's fireEvent, which already wraps in act())
          // is reflected immediately — the per-contact list is a disclosure
          // affordance a user (and this suite) expects to see update at once.
          flushSync(() => setExpanded((e) => !e));
        }}
        className="mt-gf-md text-gf-caption text-gf-accent underline"
      >
        {expanded
          ? "Hide the per-contact scores behind this"
          : "Show the per-contact scores behind this"}
      </button>
      {expanded && (
        <ul className="mt-gf-sm flex flex-col gap-gf-xs">
          {/* The top/strongest contact is already named above in the "Strongest
              contact" line — this disclosure lists the rest of the contacts
              behind the MAX so their name isn't duplicated on screen. */}
          {contacts
            .filter((c) => c.id !== orgStrength.top_person_id)
            .map((c) => (
              <li
                key={c.id}
                className="flex items-center justify-between text-gf-caption text-gf-secondary"
              >
                <span>
                  {c.data?.full_name ??
                    (c.isLoading ? "Loading…" : "Unknown contact")}
                </span>
                <span>
                  {c.data?.strength
                    ? `${c.data.strength.score}/100`
                    : "no signal yet"}
                </span>
              </li>
            ))}
        </ul>
      )}
    </div>
  );
}
