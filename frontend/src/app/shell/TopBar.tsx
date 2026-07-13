import type { ReactNode } from "react";

export type TopBarAction = { id: string; render: () => ReactNode };

export function TopBar({
  title,
  actions,
}: {
  title?: string;
  actions?: TopBarAction[];
}) {
  return (
    <header
      data-testid="top-bar"
      className="flex h-14 shrink-0 items-center justify-between border-b border-gf-subtle bg-gf-elevated px-gf-md"
    >
      <h1 className="font-display text-base font-semibold text-gf-primary">
        {title}
      </h1>
      {/* Contextual action area — renders ONLY actions true for the current
          state. Empty at cold start ("nothing connected", §2b). */}
      <div
        data-testid="top-bar-actions"
        role="toolbar"
        aria-label="Page actions"
        className="flex items-center gap-gf-sm"
      >
        {(actions ?? []).map((a) => (
          <span key={a.id}>{a.render()}</span>
        ))}
      </div>
    </header>
  );
}
