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

  it("renders raw/weighted from the server rollup, not a client-side sum (DEAL-EXT-1)", () => {
    // Two deals in DIFFERENT currencies — a client-side reduce() over amount_minor would
    // silently add EUR + USD as if they were the same unit and mislabel the total with
    // whichever deal happens to be first. The rollup's per-stage decomposition is already
    // in the workspace's base currency, so it must drive this line instead.
    const mixedCurrencyDeals: Deal[] = [
      deals[0],
      {
        ...deals[0],
        id: "d2",
        name: "Globex",
        currency: "USD",
        amount_minor: 500_00,
      },
    ];
    render(
      <StageColumn
        stage={stage}
        deals={mixedCurrencyDeals}
        rollupStage={{
          stage_id: "s0",
          unweighted_minor: 999_00,
          weighted_minor: 111_00,
          deal_count: 2,
        }}
        baseCurrency="GBP"
        onCardClick={vi.fn()}
      />,
    );
    // Server-computed GBP totals appear verbatim...
    expect(screen.getByText(/999[.,]00/)).toBeInTheDocument();
    expect(screen.getByText(/111[.,]00/)).toBeInTheDocument();
    // ...and no client-summed EUR/USD figure (600.00, the naive sum) leaks through.
    expect(screen.queryByText(/600[.,]00/)).not.toBeInTheDocument();
  });

  it("shows an honest placeholder (not a fabricated total) when the rollup hasn't loaded yet", () => {
    render(<StageColumn stage={stage} deals={deals} onCardClick={vi.fn()} />);
    expect(screen.getByTestId("stage-column-s0")).toHaveTextContent(
      /1 deal.*—/,
    );
  });
});
