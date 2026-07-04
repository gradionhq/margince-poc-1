import { useState } from "react";

type OrgStrength = {
  score: number;
  bucket: string;
  top_person_id: string;
  top_person_name: string;
};

const bucketBarColor: Record<string, string> = {
  strong: "bg-gf-accent",
  moderate: "bg-gf-status-warning",
  weak: "bg-gf-status-danger",
};

export function OrgStrengthCell({
  strength,
  contactCount,
}: {
  strength: OrgStrength | null;
  contactCount: number;
}) {
  const [showPopover, setShowPopover] = useState(false);

  if (!strength) {
    return (
      <div data-testid="org-strength-cell" className="flex flex-col gap-0.5">
        <span className="text-gf-caption text-gf-muted italic">
          no signal yet
        </span>
      </div>
    );
  }

  const barColor = bucketBarColor[strength.bucket] ?? "bg-gf-secondary";

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: strength cell needs mouse/focus tracking for informational popover
    <div
      data-testid="org-strength-cell"
      className="relative flex flex-col gap-0.5 cursor-default"
      onMouseEnter={() => setShowPopover(true)}
      onMouseLeave={() => setShowPopover(false)}
      onFocus={() => setShowPopover(true)}
      onBlur={() => setShowPopover(false)}
    >
      <div className="flex items-center gap-gf-sm">
        <div className="w-16 h-1.5 rounded-full bg-gf-subtle overflow-hidden">
          <div
            className={`h-full rounded-full ${barColor}`}
            style={{ width: `${strength.score}%` }}
          />
        </div>
        <span className="text-gf-body font-semibold">{strength.score}</span>
      </div>
      <span className="text-gf-caption text-gf-muted">
        max over {contactCount} contacts
      </span>

      {showPopover && (
        <div className="absolute bottom-full left-0 mb-1 z-50 w-64 rounded-md bg-gf-elevated border border-gf-subtle shadow-md p-gf-md text-gf-caption">
          <p className="font-semibold text-gf-body mb-1">
            Org strength {strength.score}
          </p>
          <p className="text-gf-secondary mb-1">
            Top contact: {strength.top_person_name}
          </p>
          <p className="text-gf-muted italic">
            the org score is the max over its contacts — not an average that
            hides one warm champion.
          </p>
        </div>
      )}
    </div>
  );
}
