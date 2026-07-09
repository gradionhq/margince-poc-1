import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { QuotaAttainment } from "../api/quotas.js";
import { QuotaExplainBox } from "./QuotaExplainBox.js";

const ATTAINMENT: QuotaAttainment = {
  quota_id: "q1",
  closed_won_minor: 31387200,
  target_minor: 28000000,
  currency: "EUR",
  attainment_pct: 112,
  gap_minor: 3387200,
  pace_pct: 64,
  band: "met",
  as_of_date: "2026-08-20",
  contributing_deals: [
    { deal_id: "d1", base_value_minor: 17707200 },
    { deal_id: "d2", base_value_minor: 9450000 },
    { deal_id: "d3", base_value_minor: 4230000 },
  ],
};

describe("QuotaExplainBox", () => {
  it("shows a 'computed server-side' provenance chip", () => {
    render(<QuotaExplainBox attainment={ATTAINMENT} />);
    expect(screen.getByText(/computed server-side/i)).toBeInTheDocument();
  });

  it("AC-quota-4: toggling 'Explain this number' reveals the formula, the summed deal values, and the flagged human-set target", async () => {
    render(<QuotaExplainBox attainment={ATTAINMENT} />);
    expect(screen.queryByTestId("quota-explain-box-content")).not.toBeInTheDocument();
    await userEvent.click(screen.getByText(/explain this number/i));
    const box = screen.getByTestId("quota-explain-box-content");
    expect(box).toBeInTheDocument();
    expect(box.textContent).toMatch(/Σ\(closed-won base_value\) ÷ target/);
    expect(box.textContent).toMatch(/human-set/i);
    expect(box.textContent).toMatch(/112%/);
  });
});
