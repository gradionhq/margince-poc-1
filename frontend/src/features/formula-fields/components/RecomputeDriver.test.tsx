import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { RecomputeDriver } from "./RecomputeDriver.js";

describe("RecomputeDriver", () => {
  it("switches scenarios, flashes the expected delta, and shows a simulation toast", () => {
    const onFlash = vi.fn();

    render(
      <RecomputeDriver
        openPipeline={{
          key: "open_pipeline",
          label: "Open pipeline",
          kind: "currency_minor",
          value_minor: 21200000,
          formula_sql: "sum(deal.amount_minor_base)",
          dependencies: ["deal.amount_minor", "deal.fx_rate_to_base"],
          computable: true,
          reason: null,
        }}
        onFlash={onFlash}
      />,
    );

    expect(screen.getByText("40%")).toBeInTheDocument();
    expect(screen.getByText("212.000,00 €")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "177k" }));

    expect(onFlash).toHaveBeenCalledWith(-3500000);
    expect(screen.getByText("177.000,00 €")).toBeInTheDocument();
    expect(
      screen.getByText(/simulation only -35\.000,00 €\. nothing is saved\./i),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "lost" }));

    expect(onFlash).toHaveBeenLastCalledWith(-21200000);
    expect(
      screen.getByText(/simulation only - the open pipeline drops to zero\./i),
    ).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "lost" })).toBeChecked();
    expect(screen.getByText("0,00 €")).toBeInTheDocument();
  });
});
