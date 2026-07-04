import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StageHistoryCard } from "./StageHistoryCard.js";

describe("StageHistoryCard", () => {
  it("lists create + advance_stage entries with date + summary, creation row included", () => {
    render(
      <StageHistoryCard
        entries={
          [
            {
              id: "h1",
              action: "create",
              occurred_at: "2026-01-01T00:00:00Z",
              summary: "Devin created the deal",
            },
            {
              id: "h2",
              action: "advance_stage",
              occurred_at: "2026-01-03T00:00:00Z",
              summary: "Devin changed Stage from Discovery to Proposal",
            },
          ] as never[]
        }
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("Devin created the deal")).toBeInTheDocument();
    expect(
      screen.getByText("Devin changed Stage from Discovery to Proposal"),
    ).toBeInTheDocument();
  });

  it("honest-empty when there is no history", () => {
    render(<StageHistoryCard entries={[]} isLoading={false} isError={false} />);
    expect(screen.getByText(/no stage history yet/i)).toBeInTheDocument();
  });
});
