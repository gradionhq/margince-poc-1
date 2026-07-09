import { useRef, useState } from "react";
import type { components } from "../../../lib/api-client/generated/index.js";
import { PopoverPortal } from "../../../shared/ui/forge.js";
import { formatComputedFieldValue } from "../lib/format.js";

type ComputedField = components["schemas"]["ComputedField"];

export function ExplainBox({ field }: { field: ComputedField }) {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement>(null);

  return (
    <div>
      <button
        ref={anchorRef}
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="text-gf-caption text-gf-accent underline"
      >
        Explain this number
      </button>
      {open && (
        <PopoverPortal
          anchorRef={anchorRef}
          placement="bottom-left"
          onClickOutside={() => setOpen(false)}
        >
          <div
            data-testid={`formula-field-explain-${field.key}`}
            className="max-w-sm rounded-md border border-gf-subtle bg-gf-card p-gf-md text-gf-body text-gf-primary shadow-lg"
          >
            <p className="text-gf-caption text-gf-secondary">{field.label}</p>
            {field.computable ? (
              <div className="mt-gf-sm flex flex-col gap-gf-sm">
                <div>
                  <p className="text-gf-caption text-gf-secondary">Formula</p>
                  <pre className="mt-1 overflow-x-auto rounded-md bg-gf-elevated px-gf-sm py-1 font-mono text-xs text-gf-primary">
                    {field.formula_sql}
                  </pre>
                </div>
                <div>
                  <p className="text-gf-caption text-gf-secondary">
                    Dependencies
                  </p>
                  <ul className="mt-1 flex flex-col gap-1">
                    {field.dependencies.map((dependency) => (
                      <li
                        key={dependency}
                        className="text-gf-caption text-gf-secondary"
                      >
                        Input from{" "}
                        <span className="font-mono">{dependency}</span>
                      </li>
                    ))}
                  </ul>
                </div>
                <div className="rounded-md bg-gf-accent/10 px-gf-sm py-gf-sm">
                  <p className="text-gf-caption text-gf-secondary">Result</p>
                  <p className="font-mono font-semibold text-gf-primary">
                    {formatComputedFieldValue(field)}
                  </p>
                </div>
              </div>
            ) : (
              <p className="mt-gf-sm text-gf-caption text-gf-secondary">
                Not computable yet — {field.reason ?? "not_yet_built"}
              </p>
            )}
          </div>
        </PopoverPortal>
      )}
    </div>
  );
}
