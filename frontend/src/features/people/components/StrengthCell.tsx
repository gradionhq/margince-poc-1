import { useRef, useState } from "react";
import { PopoverPortal } from "../../../shared/ui/forge.js";
import { FactorBar } from "./FactorBar.js";

const BUCKET_COLOR: Record<string, string> = {
  strong: "bg-gf-accent",
  moderate: "bg-gf-status-warning",
  weak: "bg-gf-status-danger",
};

type StrengthCellProps =
  | {
      noSignalYet: true;
      score?: never;
      bucket?: never;
      recency?: never;
      frequency?: never;
      reciprocity?: never;
    }
  | {
      noSignalYet?: false;
      score: number;
      bucket: "weak" | "moderate" | "strong";
      recency: number;
      frequency: number;
      reciprocity: number;
    };

export function StrengthCell(props: StrengthCellProps) {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement>(null);

  if (props.noSignalYet) {
    return (
      <div
        data-testid="strength-cell"
        className="text-gf-caption text-gf-secondary italic"
      >
        no signal yet
      </div>
    );
  }

  const { score, bucket, recency, frequency, reciprocity } = props;
  const barColor = BUCKET_COLOR[bucket] ?? "bg-gf-secondary";

  return (
    <button
      ref={anchorRef}
      type="button"
      data-testid="strength-cell"
      className="flex flex-col gap-0.5 cursor-default select-none text-left"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onFocus={() => setOpen(true)}
      onBlur={() => setOpen(false)}
    >
      <div className="flex items-center gap-gf-xs">
        <div className="w-16 h-1.5 bg-gf-subtle rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full ${barColor}`}
            style={{ width: `${score}%` }}
          />
        </div>
        <span className="text-gf-caption font-medium text-gf-primary">
          {score}
        </span>
      </div>
      <span className="text-gf-label text-gf-secondary">
        recency·frequency·reciprocity
      </span>
      {open && (
        <PopoverPortal
          anchorRef={anchorRef}
          placement="bottom-left"
          onClickOutside={() => setOpen(false)}
        >
          <div className="flex flex-col gap-gf-sm p-gf-md bg-gf-elevated rounded-md shadow-sm min-w-48">
            <div className="text-gf-body font-semibold text-gf-primary">
              Relationship-strength {score}
            </div>
            <div className="flex flex-col gap-gf-xs">
              <FactorBar label="Recency" value={recency} />
              <FactorBar label="Frequency" value={frequency} />
              <FactorBar label="Reciprocity" value={reciprocity} />
            </div>
            <p className="text-gf-label text-gf-secondary">
              Computed from the captured timeline — never a black-box badge.
            </p>
          </div>
        </PopoverPortal>
      )}
    </button>
  );
}
