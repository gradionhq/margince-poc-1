import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { PipelineRollup } from "../../../lib/api-client/generated/index.js";
import { TotalsStrip } from "./TotalsStrip.js";

const rollup: PipelineRollup = {
  pipeline_id: "p1",
  unweighted_minor: 146_000_00,
  weighted_minor: 96_800_00,
  base_currency: "EUR",
  as_of_date: "2026-06-04",
  by_stage: [],
  breakdown: [],
};

describe("TotalsStrip", () => {
  it("shows Weighted, Raw pipeline, Open deals reading only the roll-up", () => {
    render(<TotalsStrip rollup={rollup} isLoading={false} isError={false} />);
    expect(screen.getByText(/weighted/i)).toBeInTheDocument();
    expect(screen.getByText(/raw pipeline/i)).toBeInTheDocument();
    expect(screen.getByText(/open deals/i)).toBeInTheDocument();
  });

  it("renders a loading skeleton, not stale numbers", () => {
    render(<TotalsStrip rollup={undefined} isLoading={true} isError={false} />);
    expect(screen.getByTestId("totals-strip-skeleton")).toBeInTheDocument();
  });

  it("renders an honest error state with no fabricated numbers", () => {
    render(<TotalsStrip rollup={undefined} isLoading={false} isError={true} />);
    expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
  });
});
