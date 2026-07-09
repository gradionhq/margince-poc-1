import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { QuotaAttainment } from "../api/quotas.js";

vi.mock("../api/quotas.js", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/quotas.js")>();
  return {
    ...actual,
    useContributingDealDetails: vi.fn(),
  };
});

import { useContributingDealDetails } from "../api/quotas.js";
import { ContributingDealsTable } from "./ContributingDealsTable.js";

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
  ],
};

beforeEach(() => {
  (useContributingDealDetails as ReturnType<typeof vi.fn>).mockReturnValue([
    {
      id: "d1",
      data: {
        id: "d1",
        name: "BÄR Pharma — Packaging QA",
        closed_at: "2026-08-14T00:00:00Z",
      },
      isLoading: false,
      isError: false,
    },
    {
      id: "d2",
      data: {
        id: "d2",
        name: "Brandt — Line QA Retrofit",
        closed_at: "2026-07-29T00:00:00Z",
      },
      isLoading: false,
      isError: false,
    },
  ]);
});

describe("ContributingDealsTable", () => {
  it("AC-quota-5: lists each closed-won deal with name, close date, a Closed-won pill, and counted amount", () => {
    render(<ContributingDealsTable attainment={ATTAINMENT} />);
    expect(screen.getByText("BÄR Pharma — Packaging QA")).toBeInTheDocument();
    expect(screen.getByText("2026-08-14")).toBeInTheDocument();
    expect(screen.getAllByText(/closed-won/i).length).toBeGreaterThan(0);
    expect(screen.getByText(/177\.072,00/)).toBeInTheDocument();
  });

  it("AC-quota-5: the footer sums the counted total from closed_won_minor (never re-summed client-side)", () => {
    render(<ContributingDealsTable attainment={ATTAINMENT} />);
    expect(screen.getByText(/313\.872,00/)).toBeInTheDocument();
  });

  it("AC-quota-5: notes open/lost/omitted deals are excluded", () => {
    render(<ContributingDealsTable attainment={ATTAINMENT} />);
    expect(screen.getByText(/excluded/i)).toBeInTheDocument();
  });
});
