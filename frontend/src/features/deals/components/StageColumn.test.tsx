import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Deal, Stage } from "../../../lib/api-client/generated/index.js";
import { StageColumn } from "./StageColumn.js";

const stage: Stage = {
  id: "s0",
  workspace_id: "w1",
  pipeline_id: "p1",
  name: "New",
  position: 0,
  semantic: "open",
  win_probability: 10,
  created_at: "",
  updated_at: "",
};

const deals: Deal[] = [
  {
    id: "d1",
    workspace_id: "w1",
    name: "Acme",
    amount_minor: 100_00,
    currency: "EUR",
    pipeline_id: "p1",
    stage_id: "s0",
    status: "open",
    source: "manual",
    captured_by: "human:u1",
    created_at: "",
    updated_at: "",
    stakeholder_count: 1,
    stalled: false,
    stage_entered_at: "",
  },
];

describe("StageColumn", () => {
  it("renders the stage header and its deals", () => {
    render(<StageColumn stage={stage} deals={deals} onCardClick={vi.fn()} />);
    expect(screen.getByTestId("stage-column-s0")).toBeInTheDocument();
    expect(screen.getByText(/New/)).toBeInTheDocument();
    expect(screen.getByText("Acme")).toBeInTheDocument();
  });

  it("renders an empty column as a valid drop target, not collapsed", () => {
    render(<StageColumn stage={stage} deals={[]} onCardClick={vi.fn()} />);
    expect(screen.getByTestId("stage-column-s0")).toBeInTheDocument();
  });
});
