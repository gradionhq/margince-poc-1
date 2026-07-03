import type { ReactNode } from "react";

interface FieldGuardProps {
  mode: "visible" | "masked" | "readonly";
  children: ReactNode;
}

export function FieldGuard({ mode, children }: FieldGuardProps) {
  if (mode === "masked") {
    // A masked field must read as "withheld", not "absent": show a visible mask
    // token rather than omitting the node (which is indistinguishable from no data).
    return (
      <span
        role="img"
        data-testid="field-guard-masked"
        className="text-gf-body text-gf-secondary select-none tracking-widest"
        aria-label="Masked value"
      >
        ••••
      </span>
    );
  }
  if (mode === "readonly") {
    return (
      // biome-ignore lint/a11y/useAriaPropsSupportedByRole: signals read-only field state to assistive tech; the generic span is the masked-value wrapper.
      <span
        className="text-gf-body text-gf-secondary select-none"
        aria-readonly="true"
      >
        {children}
      </span>
    );
  }
  return <>{children}</>;
}
