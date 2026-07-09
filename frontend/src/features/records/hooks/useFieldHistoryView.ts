import { useMemo, useState } from "react";
import {
  type FieldHistoryEntry,
  humanizeFieldKey,
  scalarFieldKeys,
} from "../api/fieldHistory.js";

export interface FieldHistoryGroup {
  field: string;
  label: string;
  currentValue: unknown;
  allEntries: FieldHistoryEntry[];
  visibleEntries: FieldHistoryEntry[];
}

export type ActorFilter = "all" | "human" | "agent";

export function useFieldHistoryView(
  entries: FieldHistoryEntry[],
  record: Record<string, unknown> | undefined,
) {
  const [actor, setActor] = useState<ActorFilter>("all");
  const [field, setField] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  const fieldKeys = useMemo(() => {
    const catalog = new Set(scalarFieldKeys(record));
    for (const e of entries) catalog.add(e.field);
    return Array.from(catalog).sort();
  }, [record, entries]);

  const header = useMemo(
    () => ({ fieldCount: fieldKeys.length, changeCount: entries.length }),
    [fieldKeys, entries],
  );

  const groups: FieldHistoryGroup[] = useMemo(
    () =>
      fieldKeys.map((f) => {
        const allEntries = entries
          .filter((e) => e.field === f)
          .sort((a, b) => b.changed_at.localeCompare(a.changed_at));
        const visibleEntries =
          actor === "all"
            ? allEntries
            : allEntries.filter((e) => e.actor_type === actor);
        return {
          field: f,
          label: humanizeFieldKey(f),
          currentValue: record ? record[f] : undefined,
          allEntries,
          visibleEntries,
        };
      }),
    [fieldKeys, entries, actor, record],
  );

  const filteredGroups = useMemo(
    () =>
      groups.filter((g) => {
        if (field && g.field !== field) return false;
        if (
          actor !== "all" &&
          g.allEntries.length > 0 &&
          g.visibleEntries.length === 0
        ) {
          return false;
        }
        if (
          search &&
          !g.label.toLowerCase().includes(search.trim().toLowerCase())
        )
          return false;
        return true;
      }),
    [groups, field, actor, search],
  );

  const hasActiveFilters = actor !== "all" || field !== null || search !== "";

  function clearFilters() {
    setActor("all");
    setField(null);
    setSearch("");
  }

  return {
    header,
    groups,
    filteredGroups,
    actor,
    setActor,
    field,
    setField,
    search,
    setSearch,
    hasActiveFilters,
    clearFilters,
  };
}
