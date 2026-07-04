// KNOWN CONTRACT GAP (Global Constraints #5): ActivityRef carries no source/captured_by of its
// own (crm.d.ts — only id/kind/subject/occurred_at), so every row below is tagged with the ORG's
// own provenance, not true per-row provenance. Flagged in the PR description.
import type { components } from "../../../lib/api-client/generated/index.js";
import { SectionHeader } from "../../../shared/ui/forge.js";
import { SourceChip } from "../../people/components/SourceChip.js";

type ActivityRef = components["schemas"]["ActivityRef"];

export function ActivityCard({
  activities,
  source,
  capturedBy,
}: {
  activities: ActivityRef[];
  source: string;
  capturedBy: string;
}) {
  return (
    <div className="p-gf-lg rounded-lg border border-gf-subtle bg-gf-card">
      <SectionHeader label="Activity" />
      {activities.length === 0 ? (
        <p className="mt-gf-sm text-gf-body text-gf-muted">No activity yet.</p>
      ) : (
        <ul className="mt-gf-sm flex flex-col gap-gf-sm">
          {activities.map((a) => (
            <li key={a.id} className="flex items-center justify-between">
              <span className="text-gf-body text-gf-primary">
                {a.subject ?? a.kind}
              </span>
              <SourceChip source={source} capturedBy={capturedBy} />
            </li>
          ))}
        </ul>
      )}
      <p className="mt-gf-md text-gf-caption text-gf-muted italic">
        You logged none of this — capture linked every item.
      </p>
    </div>
  );
}
