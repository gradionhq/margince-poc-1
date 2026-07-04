import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Deal, Stage } from "../../../lib/api-client/generated/index.js";
import { PipelineBoard } from "./PipelineBoard.js";

// Vitest hoists vi.mock factories above imports, but allows referencing identifiers prefixed
// `mock` inside them — declared once here so every test in this file (including later tasks'
// Advance-button tests) can assert against the same mutate spy instead of each declaring its own.
const mockAdvanceMutate = vi.fn();
vi.mock("../api/deals.js", () => ({
  useAdvanceDeal: () => ({ mutate: mockAdvanceMutate, isPending: false }),
}));

const stages: Stage[] = [
  {
    id: "s0",
    workspace_id: "w1",
    pipeline_id: "p1",
    name: "New",
    position: 0,
    semantic: "open",
    win_probability: 10,
    created_at: "",
    updated_at: "",
  },
  {
    id: "s1",
    workspace_id: "w1",
    pipeline_id: "p1",
    name: "Qualified",
    position: 1,
    semantic: "open",
    win_probability: 25,
    created_at: "",
    updated_at: "",
  },
];

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

describe("PipelineBoard", () => {
  it("groups cards into stage columns with header name/probability/count", () => {
    render(
      <PipelineBoard
        pipelineId="p1"
        stages={stages}
        deals={deals}
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
        onCardClick={vi.fn()}
      />,
    );
    expect(screen.getByText(/New/)).toBeInTheDocument();
    expect(screen.getByText(/10%/)).toBeInTheDocument();
    expect(screen.getByText(/Qualified/)).toBeInTheDocument();
    expect(screen.getByText("Acme")).toBeInTheDocument();
  });

  it("renders empty stage columns as valid drop targets, not collapsed (STATE-1)", () => {
    render(
      <PipelineBoard
        pipelineId="p1"
        stages={stages}
        deals={[]}
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
        onCardClick={vi.fn()}
      />,
    );
    expect(screen.getByTestId("stage-column-s0")).toBeInTheDocument();
    expect(screen.getByTestId("stage-column-s1")).toBeInTheDocument();
  });

  it("renders an honest top-level empty state on a zero-deal board (STATE-1)", () => {
    render(
      <PipelineBoard
        pipelineId="p1"
        stages={stages}
        deals={[]}
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
        onCardClick={vi.fn()}
      />,
    );
    expect(screen.getByTestId("board-empty-state")).toBeInTheDocument();
  });

  it("renders loading skeleton (STATE-2)", () => {
    render(
      <PipelineBoard
        pipelineId="p1"
        stages={stages}
        deals={[]}
        isLoading={true}
        isError={false}
        onRetry={vi.fn()}
        onCardClick={vi.fn()}
      />,
    );
    expect(screen.getByTestId("board-skeleton")).toBeInTheDocument();
  });

  it("renders honest error with retry (STATE-3)", () => {
    const onRetry = vi.fn();
    render(
      <PipelineBoard
        pipelineId="p1"
        stages={stages}
        deals={[]}
        isLoading={false}
        isError={true}
        onRetry={onRetry}
        onCardClick={vi.fn()}
      />,
    );
    screen.getByRole("button", { name: /retry/i }).click();
    expect(onRetry).toHaveBeenCalled();
  });
});

describe("PipelineBoard drag mechanics", () => {
  it("navigates on a plain click (no drag)", async () => {
    const onCardClick = vi.fn();
    render(
      <PipelineBoard
        pipelineId="p1"
        stages={stages}
        deals={deals}
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
        onCardClick={onCardClick}
      />,
    );
    await userEvent.click(screen.getByTestId("deal-card-d1"));
    expect(onCardClick).toHaveBeenCalledWith("d1");
  });
});

// dnd-kit's pointer-sensor activation-distance behavior is inherently hard to unit-test through
// jsdom pointer events with full fidelity — the click-vs-drag-tail distinction itself is
// asserted at the live-UAT layer (workspace/manual-test/t21.md step 4). The test above only pins
// the non-drag click path so a regression here is still caught fast.

const wonStage: Stage = {
  id: "sw",
  workspace_id: "w1",
  pipeline_id: "p1",
  name: "Closed Won",
  position: 5,
  semantic: "won",
  win_probability: 100,
  created_at: "",
  updated_at: "",
};
const lostStage: Stage = {
  id: "sl",
  workspace_id: "w1",
  pipeline_id: "p1",
  name: "Closed Lost",
  position: 6,
  semantic: "lost",
  win_probability: 0,
  created_at: "",
  updated_at: "",
};

describe("PipelineBoard terminal transitions", () => {
  it("does not render Won/Lost as standing columns absent an active drag (DEAL-AC-B3)", () => {
    render(
      <PipelineBoard
        pipelineId="p1"
        stages={stages}
        terminalStages={[wonStage, lostStage]}
        deals={deals}
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
        onCardClick={vi.fn()}
      />,
    );
    expect(screen.queryByText("Closed Won")).not.toBeInTheDocument();
    expect(screen.queryByText("Closed Lost")).not.toBeInTheDocument();
  });
});

describe("PipelineBoard Advance button", () => {
  it("open→open Advance click applies directly, no confirm dialog (🟢)", () => {
    mockAdvanceMutate.mockClear();
    render(
      <PipelineBoard
        pipelineId="p1"
        stages={stages} // s0 "New" (pos 0), s1 "Qualified" (pos 1) — both open
        deals={deals} // d1 sits in s0
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
        onCardClick={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^advance$/i }));
    expect(mockAdvanceMutate).toHaveBeenCalledWith({
      dealId: "d1",
      toStageId: "s1",
    });
    expect(screen.queryByText(/confirm the outcome/i)).not.toBeInTheDocument();
  });

  it("Advance click when the next stage is terminal opens the outcome dialog (🟡)", () => {
    const lastStage: Stage = { ...stages[1], id: "s1" }; // deal already in the last open stage
    const dealAtLastStage: Deal = { ...deals[0], stage_id: "s1" };
    render(
      <PipelineBoard
        pipelineId="p1"
        stages={[stages[0], lastStage]}
        terminalStages={[wonStage, lostStage]}
        deals={[dealAtLastStage]}
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
        onCardClick={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^advance$/i }));
    expect(screen.getByText(/confirm the outcome/i)).toBeInTheDocument();
  });
});
