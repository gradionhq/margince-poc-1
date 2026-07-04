import type { components } from "../../../lib/api-client/generated/index.js";

type ActivityRef = components["schemas"]["ActivityRef"];

// KNOWN CONTRACT GAP (flag in the PR description, mirrors the merge/notes gaps): AC-person-8
// asks for a per-row "SourceChip-style" provenance chip (e.g. "connector · Gmail thread"), but
// `ActivityRef` (the shape of Person.activities on the composite read) carries only
// id/kind/subject/occurred_at — no source/captured_by. Rendering a chip here would mean
// fabricating provenance data that isn't actually on the wire. We render kind/subject/date
// honestly and omit the per-row chip; the fixed caption below is the spec's own mandated copy,
// not a fabricated claim about any specific row.
export function ActivityTab({ activities }: { activities: ActivityRef[] }) {
  if (activities.length === 0) {
    return (
      <p data-testid="activity-tab-empty" className="text-gf-body text-gf-secondary">
        No activity captured yet.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-gf-sm">
      {activities.map((a) => (
        <div
          key={a.id}
          className="flex items-center justify-between border border-gf-subtle rounded-md p-gf-sm"
        >
          <div className="flex flex-col">
            <span className="text-gf-caption font-medium text-gf-primary capitalize">
              {a.kind}
            </span>
            <span className="text-gf-body text-gf-primary">{a.subject ?? "(no subject)"}</span>
          </div>
          <span className="text-gf-label text-gf-secondary">
            {new Date(a.occurred_at).toISOString().slice(0, 10)}
          </span>
        </div>
      ))}
      <p className="text-gf-label text-gf-secondary italic">
        You logged none of this — every row carries its source.
      </p>
    </div>
  );
}
