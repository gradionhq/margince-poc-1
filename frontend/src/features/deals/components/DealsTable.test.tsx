import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Deal, Stage } from "../../../lib/api-client/generated/index.js";
import { DealsTable } from "./DealsTable.js";

const stage: Stage = { id: "s1", workspace_id: "w1", pipeline_id: "p1", name: "Proposal", position: 3, semantic: "open", win_probability: 60, created_at: "", updated_at: "" };
const deals: Deal[] = [
  { id: "d1", workspace_id: "w1", name: "Big deal", amount_minor: 200_00, currency: "EUR", pipeline_id: "p1", stage_id: "s1", status: "open", source: "manual", captured_by: "human:u1", created_at: "", updated_at: "", stalled: true, stakeholder_count: 2, stage_entered_at: "2020-01-01T00:00:00Z" },
  { id: "d2", workspace_id: "w1", name: "Small deal", amount_minor: 50_00, currency: "EUR", pipeline_id: "p1", stage_id: "s1", status: "open", source: "manual", captured_by: "human:u1", created_at: "", updated_at: "", stalled: false, stakeholder_count: 2, stage_entered_at: "2026-07-01T00:00:00Z" },
];

describe("DealsTable", () => {
  it("sorts rows by amount desc and shows the stage pill", () => {
    render(<DealsTable deals={deals} stagesById={{ s1: stage }} />);
    const rows = screen.getAllByRole("row");
    // header row + 2 data rows; first data row should be the bigger amount
    expect(rows[1]).toHaveTextContent("Big deal");
    expect(rows[2]).toHaveTextContent("Small deal");
    expect(screen.getAllByText("Proposal")).toHaveLength(2);
  });

  it("marks a stalled deal's Age cell amber with a warning icon (AC-pipeline-8)", () => {
    render(<DealsTable deals={deals} stagesById={{ s1: stage }} />);
    expect(screen.getByTestId("age-cell-d1").className).toMatch(/warning/i);
  });
});
