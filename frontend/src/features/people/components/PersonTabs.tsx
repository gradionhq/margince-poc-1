import { useState } from "react";
import type { components } from "../../../lib/api-client/generated/index.js";
import { ActivityTab } from "./ActivityTab.js";
import { NotesTab } from "./NotesTab.js";
import { PersonDealsTab } from "./PersonDealsTab.js";

type ActivityRef = components["schemas"]["ActivityRef"];
type TabKey = "activity" | "deals" | "notes";

const TABS: { key: TabKey; label: string }[] = [
  { key: "activity", label: "Activity" },
  { key: "deals", label: "Deals" },
  { key: "notes", label: "Notes" },
];

export function PersonTabs({
  personId,
  activities,
}: {
  personId: string;
  activities: ActivityRef[];
}) {
  const [active, setActive] = useState<TabKey>("activity");

  return (
    <div className="flex flex-col gap-gf-sm">
      <div role="tablist" className="flex gap-gf-sm border-b border-gf-subtle">
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            role="tab"
            aria-selected={active === t.key}
            onClick={() => setActive(t.key)}
            className={`px-gf-sm py-gf-xs text-gf-body ${
              active === t.key
                ? "text-gf-primary border-b-2 border-gf-accent"
                : "text-gf-secondary"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div className="pt-gf-sm">
        {active === "activity" && <ActivityTab activities={activities} />}
        {active === "deals" && <PersonDealsTab personId={personId} />}
        {active === "notes" && <NotesTab />}
      </div>
    </div>
  );
}
