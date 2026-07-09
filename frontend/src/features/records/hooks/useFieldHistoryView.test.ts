import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { FieldHistoryEntry } from "../api/fieldHistory.js";
import { useFieldHistoryView } from "./useFieldHistoryView.js";

function entry(overrides: Partial<FieldHistoryEntry>): FieldHistoryEntry {
  return {
    id: "e1", entity_type: "deal", entity_id: "d1", field: "amount_minor",
    old_value: null, new_value: "1000", changed_at: "2026-01-01T00:00:00Z",
    actor_type: "human", actor_id: "u1", passport_id: null, evidence: null,
    ...overrides,
  };
}

const RECORD = { id: "d1", amount_minor: 17707200, stage_id: "s1", owner_id: "u1" };

describe("useFieldHistoryView", () => {
  it("AC-1: header counts the full field catalog and total changes, unaffected by filters", () => {
    const entries = [
      entry({ id: "e1", field: "amount_minor" }),
      entry({ id: "e2", field: "amount_minor", changed_at: "2026-02-01T00:00:00Z" }),
      entry({ id: "e3", field: "stage_id" }),
    ];
    const { result } = renderHook(() => useFieldHistoryView(entries, RECORD));
    expect(result.current.header.fieldCount).toBe(3);
    expect(result.current.header.changeCount).toBe(3);
    act(() => result.current.setActor("human"));
    expect(result.current.header.fieldCount).toBe(3);
    expect(result.current.header.changeCount).toBe(3);
  });

  it("AC-field-history-8: a field with zero total entries still gets a group, currentValue set from the record", () => {
    const { result } = renderHook(() => useFieldHistoryView([], RECORD));
    const owner = result.current.groups.find((g) => g.field === "owner_id");
    expect(owner).toBeDefined();
    expect(owner?.allEntries).toHaveLength(0);
    expect(owner?.currentValue).toBe("u1");
  });

  it("AC-3: selecting Agent hides a group whose entries are all human-authored", () => {
    const entries = [
      entry({ id: "e1", field: "amount_minor", actor_type: "human" }),
      entry({ id: "e2", field: "stage_id", actor_type: "agent" }),
    ];
    const { result } = renderHook(() => useFieldHistoryView(entries, RECORD));
    act(() => result.current.setActor("agent"));
    const fields = result.current.filteredGroups.map((g) => g.field);
    expect(fields).toContain("stage_id");
    expect(fields).not.toContain("amount_minor");
  });

  it("AC-3 (finding 7): a genuinely zero-entry group is NOT hidden by an actor filter", () => {
    const { result } = renderHook(() => useFieldHistoryView([], RECORD));
    act(() => result.current.setActor("agent"));
    expect(result.current.filteredGroups.map((g) => g.field)).toEqual(
      expect.arrayContaining(["owner_id"]),
    );
  });

  it("AC-4: selecting a field chip narrows to exactly one group; All fields restores every group", () => {
    const entries = [entry({ id: "e1", field: "amount_minor" }), entry({ id: "e2", field: "stage_id" })];
    const { result } = renderHook(() => useFieldHistoryView(entries, RECORD));
    act(() => result.current.setField("stage_id"));
    expect(result.current.filteredGroups.map((g) => g.field)).toEqual(["stage_id"]);
    act(() => result.current.setField(null));
    expect(result.current.filteredGroups.length).toBe(result.current.groups.length);
  });

  it("AC-5: a search matching no field label hides every group", () => {
    const { result } = renderHook(() => useFieldHistoryView([], RECORD));
    act(() => result.current.setSearch("zzz-no-match"));
    expect(result.current.filteredGroups).toHaveLength(0);
  });

  it("AC-5: clearFilters resets actor, field, and search together", () => {
    const { result } = renderHook(() => useFieldHistoryView([], RECORD));
    act(() => {
      result.current.setActor("agent");
      result.current.setField("owner_id");
      result.current.setSearch("own");
    });
    expect(result.current.hasActiveFilters).toBe(true);
    act(() => result.current.clearFilters());
    expect(result.current.actor).toBe("all");
    expect(result.current.field).toBeNull();
    expect(result.current.search).toBe("");
    expect(result.current.hasActiveFilters).toBe(false);
  });
});
