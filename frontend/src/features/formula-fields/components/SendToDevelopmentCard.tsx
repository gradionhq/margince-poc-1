import { useState } from "react";
import { Button } from "../../../shared/ui/forge.js";

type CardState = "proposed" | "routed" | "dismissed";

export function SendToDevelopmentCard() {
  const [state, setState] = useState<CardState>("proposed");
  const [toast, setToast] = useState<string | null>(null);

  if (state === "dismissed") return null;

  const showToast = (message: string) => setToast(message);

  return (
    <section className="rounded-xl border border-gf-subtle bg-gf-card p-gf-lg shadow-sm">
      <div className="flex flex-wrap items-start gap-gf-sm">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-gf-xs">
            <span className="inline-flex items-center rounded-full border border-gf-subtle bg-gf-elevated px-gf-xs py-0.5 text-gf-caption font-medium text-gf-secondary">
              AI-proposed
            </span>
            <span className="text-gf-caption text-gf-muted">
              Formula field proposal
            </span>
          </div>
          <h3 className="mt-gf-sm text-gf-title font-semibold text-gf-primary">
            Account health
          </h3>
          <p className="mt-gf-xs text-gf-body text-gf-secondary">
            A reviewed source change owns the logic; this screen only stages the
            handoff.
          </p>
        </div>
      </div>

      {state === "routed" ? (
        <div className="mt-gf-md rounded-lg border border-gf-subtle bg-gf-elevated p-gf-md text-gf-body text-gf-primary">
          <p>
            This logic ships as a reviewed source change, not as runtime editor
            state.
          </p>
          <p className="mt-gf-xs text-gf-caption text-gf-secondary">
            This needs the development path, not this screen.
          </p>
          <a
            href="/development"
            className="mt-gf-sm inline-flex text-gf-caption text-gf-accent underline"
          >
            Development path
          </a>
        </div>
      ) : (
        <div className="mt-gf-md rounded-lg border border-dashed border-gf-subtle bg-gf-elevated/60 p-gf-md text-gf-caption text-gf-secondary">
          Draft formula logic is reviewed source, never runtime editable here.
        </div>
      )}

      <div className="mt-gf-md flex flex-wrap gap-gf-sm">
        {state === "proposed" && (
          <Button
            variant="primary"
            onClick={() => {
              setState("routed");
              showToast("Formula logic is reviewed code, not runtime.");
            }}
          >
            Send to development
          </Button>
        )}
        <Button
          variant="secondary"
          onClick={() =>
            showToast(
              "Draft edit - formula logic ships as reviewed source, not edited here",
            )
          }
        >
          Edit formula
        </Button>
        <Button variant="ghost" onClick={() => setState("dismissed")}>
          Dismiss
        </Button>
      </div>

      {toast && (
        <div
          role="status"
          aria-live="polite"
          className="mt-gf-md rounded-md border border-gf-subtle bg-gf-page px-gf-md py-gf-sm text-gf-caption text-gf-primary"
        >
          {toast}
        </div>
      )}
    </section>
  );
}
