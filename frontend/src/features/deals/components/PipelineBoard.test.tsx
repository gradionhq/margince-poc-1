import { render, screen } from "@testing-library/react";
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
  { id: "s0", workspace_id: "w1", pipeline_id: "p1", name: "New", position: 0, semantic: "open", win_probability: 10, created_at: "", updated_at: "" },
  { id: "s1", workspace_id: "w1", pipeline_id: "p1", name: "Qualified", position: 1, semantic: "open", win_probability: 25, created_at: "", updated_at: "" },
];

const deals: Deal[] = [
  { id: "d1", workspace_id: "w1", name: "Acme", amount_minor: 100_00, currency: "EUR", pipeline_id: "p1", stage_id: "s0", status: "open", source: "manual", captured_by: "human:u1", created_at: "", updated_at: "", stakeholder_count: 1, stalled: false, stage_entered_at: "" },
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
