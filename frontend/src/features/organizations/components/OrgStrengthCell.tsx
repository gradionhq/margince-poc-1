import { useRef, useState } from "react";
import { PopoverPortal } from "../../../shared/ui/forge.js";

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
  const anchorRef = useRef<HTMLButtonElement>(null);

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
    <button
      ref={anchorRef}
      type="button"
      data-testid="org-strength-cell"
      className="flex flex-col gap-0.5 cursor-default select-none text-left"
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
        <PopoverPortal
          anchorRef={anchorRef}
          placement="bottom-left"
          onClickOutside={() => setShowPopover(false)}
        >
          <div className="flex flex-col gap-gf-sm p-gf-md bg-gf-elevated rounded-md shadow-sm min-w-48 text-gf-caption">
            <p className="font-semibold text-gf-body">
              Org strength {strength.score}
            </p>
            <p className="text-gf-secondary">
              Top contact: {strength.top_person_name}
            </p>
            <p className="text-gf-muted italic">
              the org score is the max over its contacts — not an average that
              hides one warm champion.
            </p>
          </div>
        </PopoverPortal>
      )}
    </button>
  );
}
