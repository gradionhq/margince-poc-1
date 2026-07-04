import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { WeightedValueExplainer } from "./WeightedValueExplainer.js";

describe("WeightedValueExplainer", () => {
  it("labels the win-probability KPI as stage default (deterministic)", () => {
    render(
      <WeightedValueExplainer
        amountMinor={1000000}
        currency="USD"
        winProbability={40}
        stageName="Discovery"
      />,
    );
    expect(
      screen.getByText("stage default (deterministic) · Discovery"),
    ).toBeInTheDocument();
  });

  it("expands the arithmetic popover on click and collapses on a second click", async () => {
    render(
      <WeightedValueExplainer
        amountMinor={1000000}
        currency="USD"
        winProbability={40}
        stageName="Discovery"
      />,
    );
    const trigger = screen.getByRole("button", { name: /explain this number/i });
    await userEvent.click(trigger);
    expect(screen.getByTestId("weighted-value-explainer-popover")).toBeInTheDocument();
    expect(
      screen.getByText(/\$10,000\.00 × 40% = \$4,000\.00/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Won = 100%, Lost = 0%/)).toBeInTheDocument();
    await userEvent.click(trigger);
    expect(
      screen.queryByTestId("weighted-value-explainer-popover"),
    ).not.toBeInTheDocument();
  });
});
