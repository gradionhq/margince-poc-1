import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Deal, Stage } from "../../../lib/api-client/generated/index.js";
import { DealsTable } from "./DealsTable.js";

function daysAgoISO(days: number): string {
  return new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString();
}

const stage: Stage = {
  id: "s1",
  workspace_id: "w1",
  pipeline_id: "p1",
  name: "Proposal",
  position: 3,
  semantic: "open",
  win_probability: 60,
  created_at: "",
  updated_at: "",
};
const deals: Deal[] = [
  {
    id: "d1",
    workspace_id: "w1",
    name: "Big deal",
    amount_minor: 200_00,
    currency: "EUR",
    pipeline_id: "p1",
    stage_id: "s1",
    status: "open",
    source: "manual",
    captured_by: "human:u1",
    created_at: "",
    updated_at: "",
    stalled: true,
    stakeholder_count: 2,
    stage_entered_at: "2020-01-01T00:00:00Z",
  },
  {
    id: "d2",
    workspace_id: "w1",
    name: "Small deal",
    amount_minor: 50_00,
    currency: "EUR",
    pipeline_id: "p1",
    stage_id: "s1",
    status: "open",
    source: "manual",
    captured_by: "human:u1",
    created_at: "",
    updated_at: "",
    stalled: false,
    stakeholder_count: 2,
    stage_entered_at: "2026-07-01T00:00:00Z",
  },
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

  it("shows idle days (last_activity_at), not stage age, in a stalled row's Age cell (live-UAT regression)", () => {
    const idleDeal: Deal = {
      ...deals[0],
      id: "d3",
      stalled: true,
      stage_entered_at: daysAgoISO(0),
      last_activity_at: daysAgoISO(90),
    };
    render(<DealsTable deals={[idleDeal]} stagesById={{ s1: stage }} />);
    const cell = screen.getByTestId("age-cell-d3");
    const match = cell.textContent?.match(/(\d+)d$/);
    const days = Number(match?.[1]);
    expect(days).toBeGreaterThanOrEqual(89);
    expect(days).toBeLessThanOrEqual(91);
  });

  it("exposes Archive from the row actions menu and calls onArchive with the deal id", async () => {
    const onArchive = vi.fn();
    const userEventModule = await import("@testing-library/user-event");
    const user = userEventModule.default.setup();

    render(
      <DealsTable deals={deals} stagesById={{ s1: stage }} onArchive={onArchive} />,
    );

    await user.click(
      screen.getAllByRole("button", { name: /row actions/i })[0],
    );
    await user.click(screen.getByRole("menuitem", { name: "Archive" }));

    expect(onArchive).toHaveBeenCalledWith("d1");
  });
});
