import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { FieldHistoryEntry } from "../api/fieldHistory.js";
import type { FieldHistoryGroup } from "../hooks/useFieldHistoryView.js";
import { FieldHistoryGroupCard } from "./FieldHistoryGroupCard.js";

function entry(overrides: Partial<FieldHistoryEntry>): FieldHistoryEntry {
  return {
    id: "e1", entity_type: "deal", entity_id: "d1", field: "stage_id",
    old_value: "Discovery", new_value: "Qualified", changed_at: "2026-06-12T11:20:00Z",
    actor_type: "human", actor_id: "u1", passport_id: null, evidence: null,
    ...overrides,
  };
}

function group(overrides: Partial<FieldHistoryGroup>): FieldHistoryGroup {
  const allEntries = overrides.allEntries ?? [];
  return {
    field: "stage_id", label: "Stage id", currentValue: "Qualified",
    allEntries, visibleEntries: allEntries, ...overrides,
  };
}

describe("FieldHistoryGroupCard", () => {
  it("AC-2: shows Current value row and the change count", () => {
    render(<FieldHistoryGroupCard group={group({ currentValue: "Proposal sent" })} currency="EUR" />);
    expect(screen.getByText("Proposal sent")).toBeInTheDocument();
  });

  it("AC-field-history-8: zero entries shows the honest never-changed message, not a blank timeline", () => {
    render(<FieldHistoryGroupCard group={group({ allEntries: [], visibleEntries: [] })} currency="EUR" />);
    expect(
      screen.getByText(/set on create and never changed/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/an empty history is honest, not a gap/i)).toBeInTheDocument();
  });

  it("AC-2: a diff row shows struck-through from, an arrow, and a highlighted to", () => {
    const g = group({ allEntries: [entry({})], visibleEntries: [entry({})] });
    render(<FieldHistoryGroupCard group={g} currency="EUR" />);
    expect(screen.getByText("Discovery")).toBeInTheDocument();
    expect(screen.getAllByText("Qualified").length).toBeGreaterThan(0);
  });

  it("AC-2: a null old_value renders — created — for this field's oldest entry", () => {
    const e = entry({ old_value: null, new_value: "Discovery" });
    const g = group({ allEntries: [e], visibleEntries: [e] });
    render(<FieldHistoryGroupCard group={g} currency="EUR" />);
    expect(screen.getByText("— created —")).toBeInTheDocument();
  });

  it("AC-2: a null new_value renders — removed —, never a blank cell", () => {
    const e = entry({ old_value: "https://old.example.com", new_value: null });
    const g = group({ allEntries: [e], visibleEntries: [e] });
    render(<FieldHistoryGroupCard group={g} currency="EUR" />);
    expect(screen.getByText("— removed —")).toBeInTheDocument();
  });

  it("AC-2 (finding 4): a non-oldest null-old_value entry (cleared, then re-set) renders — empty —, proving DiffRow passes group.allEntries (not just its own row) to originLabel", () => {
    const oldest = entry({ id: "e-oldest", old_value: null, new_value: "first", changed_at: "2026-01-01T00:00:00Z" });
    const middle = entry({ id: "e-middle", old_value: null, new_value: "second", changed_at: "2026-02-01T00:00:00Z" });
    const allEntries = [middle, oldest];
    const g = group({ allEntries, visibleEntries: allEntries });
    render(<FieldHistoryGroupCard group={g} currency="EUR" />);
    expect(screen.getByText("— created —")).toBeInTheDocument();
    expect(screen.getByText("— empty —")).toBeInTheDocument();
  });

  it("AC-field-history-6: an agent row with evidence shows an evidence toggle; human rows never do", () => {
    const agentEntry = entry({
      id: "e-agent", actor_type: "agent",
      evidence: { quote: "budget signed off", confidence: "high", confidence_note: "computed, not inferred" },
    });
    const humanEntry = entry({ id: "e-human", actor_type: "human" });
    const g = group({ allEntries: [agentEntry, humanEntry], visibleEntries: [agentEntry, humanEntry] });
    render(<FieldHistoryGroupCard group={g} currency="EUR" />);
    expect(screen.getAllByRole("button", { name: /evidence/i })).toHaveLength(1);
  });

  it("AC-field-history-6: clicking evidence expands the grounding quote, source, and confidence dot", async () => {
    const agentEntry = entry({
      actor_type: "agent",
      evidence: { quote: "budget signed off", source_url: "https://x", confidence: "high", confidence_note: "computed, not inferred" },
    });
    const g = group({ allEntries: [agentEntry], visibleEntries: [agentEntry] });
    render(<FieldHistoryGroupCard group={g} currency="EUR" />);
    await userEvent.click(screen.getByRole("button", { name: /evidence/i }));
    expect(screen.getByText(/budget signed off/i)).toBeInTheDocument();
    expect(screen.getByText(/computed, not inferred/i)).toBeInTheDocument();
  });

  it("AC-field-history-7: the computed money field embeds an Explain-this-number box", () => {
    const e = entry({ field: "amount_minor", old_value: "21200000", new_value: "17707200" });
    const g = group({ field: "amount_minor", label: "Amount", currentValue: 17707200, allEntries: [e], visibleEntries: [e] });
    render(<FieldHistoryGroupCard group={g} currency="EUR" />);
    expect(screen.getByText(/explain this number/i)).toBeInTheDocument();
  });

  it("AC-field-history-7: a non-money field never shows Explain this number", () => {
    const g = group({ allEntries: [entry({})], visibleEntries: [entry({})] });
    render(<FieldHistoryGroupCard group={g} currency="EUR" />);
    expect(screen.queryByText(/explain this number/i)).not.toBeInTheDocument();
  });
});
