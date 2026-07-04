import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Deal } from "../../../lib/api-client/generated/index.js";
import { DealCard, formatMoney, weightedValue } from "./DealCard.js";

const baseDeal: Deal = {
  id: "d1",
  workspace_id: "w1",
  name: "Acme deal",
  amount_minor: 100_00,
  currency: "EUR",
  pipeline_id: "p1",
  stage_id: "s1",
  status: "open",
  source: "manual",
  captured_by: "human:u1",
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  stalled: false,
  stakeholder_count: 2,
  stage_entered_at: "2026-07-01T00:00:00Z",
};

describe("formatMoney", () => {
  it("uses the deal's own currency, never hard-coded EUR", () => {
    expect(formatMoney(5_000_00, "USD")).toContain("$");
    expect(formatMoney(5_000_00, "USD")).not.toContain("€");
  });
  it("handles a null amount", () => {
    expect(formatMoney(null, "EUR")).toBe("—");
  });
});

describe("weightedValue", () => {
  it("rounds half away from zero", () => {
    // 12345.5 minor rounds to 12346 per DEAL-FORM-2's worked boundary example
    expect(weightedValue(24_691, 50)).toBe(12_346);
  });
});

describe("DealCard", () => {
  it("renders the deal name and formatted amount", () => {
    render(<DealCard deal={baseDeal} onClick={vi.fn()} />);
    expect(screen.getByText("Acme deal")).toBeInTheDocument();
    expect(screen.getByText(/100[.,]00/)).toBeInTheDocument();
  });

  it("calls onClick on a plain click", async () => {
    const onClick = vi.fn();
    render(<DealCard deal={baseDeal} onClick={onClick} />);
    screen.getByTestId("deal-card-d1").click();
    expect(onClick).toHaveBeenCalled();
  });

  it("shows the Stalled flag with a days-idle hover title when stalled (AC-pipeline-5)", () => {
    const stalledDeal: Deal = {
      ...baseDeal,
      stalled: true,
      stage_entered_at: "2020-01-01T00:00:00Z",
    };
    render(<DealCard deal={stalledDeal} onClick={vi.fn()} />);
    const flag = screen.getByText(/^Stalled \d+d$/);
    expect(flag).toBeInTheDocument();
    expect(flag).toHaveAttribute(
      "title",
      expect.stringContaining("No activity for"),
    );
  });

  it("does not show the Stalled flag when not stalled", () => {
    render(<DealCard deal={baseDeal} onClick={vi.fn()} />);
    expect(screen.queryByText(/^Stalled \d+d$/)).not.toBeInTheDocument();
  });

  it("shows the Single-threaded flag when stakeholder_count is 1 (AC-pipeline-5)", () => {
    const soloDeal: Deal = { ...baseDeal, stakeholder_count: 1 };
    render(<DealCard deal={soloDeal} onClick={vi.fn()} />);
    const flag = screen.getByText("Single-threaded");
    expect(flag).toBeInTheDocument();
    expect(flag).toHaveAttribute(
      "title",
      "Only one stakeholder captured on this deal",
    );
  });

  it("does not show the Single-threaded flag when stakeholder_count is more than 1", () => {
    render(<DealCard deal={baseDeal} onClick={vi.fn()} />);
    expect(screen.queryByText("Single-threaded")).not.toBeInTheDocument();
  });
});
