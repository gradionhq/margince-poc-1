import { useState } from "react";
import type { Person } from "../../../lib/api-client/generated/index.js";
import { usePersonStrengthBreakdown } from "../api/person.js";
import { FactorBar } from "./FactorBar.js";

type Strength = NonNullable<Person["strength"]>;

export function StrengthCard({
  personId,
  strength,
}: {
  personId: string;
  strength: Strength | null | undefined;
}) {
  const [expanded, setExpanded] = useState(false);
  const { data: breakdown } = usePersonStrengthBreakdown(personId, expanded);

  if (!strength) {
    return (
      <div
        data-testid="strength-card"
        className="bg-gf-card border border-gf-subtle rounded-md p-gf-md"
      >
        <p className="text-gf-body text-gf-secondary italic">no signal yet</p>
      </div>
    );
  }

  return (
    <div
      data-testid="strength-card"
      className="bg-gf-card border border-gf-subtle rounded-md p-gf-md flex flex-col gap-gf-sm"
    >
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-gf-body font-semibold text-gf-primary">
            Relationship strength
          </h3>
          <p className="text-gf-label text-gf-secondary">Team-wide</p>
        </div>
        <span className="inline-flex items-center px-gf-sm py-gf-xs rounded-full text-gf-caption font-medium bg-gf-card border border-gf-subtle text-gf-secondary">
          computed · deterministic from captured cadence
        </span>
      </div>
      <div className="flex items-center gap-gf-sm">
        <div className="flex-1 h-1.5 bg-gf-subtle rounded-full overflow-hidden">
          <div
            className="h-full bg-gf-accent rounded-full"
            style={{ width: `${strength.score}%` }}
          />
        </div>
        <span className="text-gf-body font-medium text-gf-primary">
          {strength.score}/100
        </span>
      </div>
      <div className="grid grid-cols-3 gap-gf-sm">
        <div>
          <FactorBar label="Recency" value={strength.recency} />
          <p className="text-gf-label text-gf-secondary">30-day half-life</p>
        </div>
        <div>
          <FactorBar label="Frequency" value={strength.frequency} />
          <p className="text-gf-label text-gf-secondary">
            saturates at 20 interactions/90d
          </p>
        </div>
        <div>
          <FactorBar label="Reciprocity" value={strength.reciprocity} />
          <p className="text-gf-label text-gf-secondary">in/out balance</p>
        </div>
      </div>
      <button
        type="button"
        onClick={() => setExpanded((e) => !e)}
        className="self-start text-gf-caption text-gf-accent hover:underline"
      >
        {expanded
          ? "Hide the activities behind this score"
          : "Show the activities behind this score"}
      </button>
      {expanded && (
        <div className="flex flex-col gap-gf-sm border-t border-gf-subtle pt-gf-sm">
          <p className="text-gf-body font-mono text-gf-primary">
            {breakdown
              ? `Score = 100 × ${breakdown.recency} × ${breakdown.frequency} × ${breakdown.reciprocity} = ${breakdown.score}`
              : "Loading evidence…"}
          </p>
          <ul className="flex flex-col gap-gf-xs">
            {(breakdown?.contributing_activities ?? []).map((a) => (
              <li key={a.id} className="text-gf-caption text-gf-secondary">
                <span>{a.subject ?? a.kind}</span> —{" "}
                {new Date(a.occurred_at).toISOString().slice(0, 10)}
              </li>
            ))}
          </ul>
          <p className="text-gf-label text-gf-secondary">
            See the Activity tab · formula §4
          </p>
        </div>
      )}
    </div>
  );
}
