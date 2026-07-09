import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { QuotaAttainment } from "../api/quotas.js";
import { AttainmentRing } from "./AttainmentRing.js";

const ATTAINMENT: QuotaAttainment = {
  quota_id: "q1",
  closed_won_minor: 31387200,
  target_minor: 28000000,
  currency: "EUR",
  attainment_pct: 112.1,
  gap_minor: 3387200,
  pace_pct: 64,
  band: "met",
  as_of_date: "2026-08-20",
  contributing_deals: [],
};

describe("AttainmentRing", () => {
  it("AC-quota-1: renders attainment %, closed-won, target, gap from the server response", () => {
    render(
      <AttainmentRing
        attainment={ATTAINMENT}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("112%")).toBeInTheDocument();
    expect(screen.getByText(/313\.872,00/)).toBeInTheDocument();
    expect(screen.getByText(/280\.000,00/)).toBeInTheDocument();
    expect(screen.getByText(/\+33\.872,00/)).toBeInTheDocument();
  });

  it("AC-quota-3: pace line reads 'Target met' at >=100%", () => {
    render(
      <AttainmentRing
        attainment={ATTAINMENT}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText(/target met/i)).toBeInTheDocument();
  });

  it("AC-quota-3: pace line reads 'Ahead of pace' when attainment_pct >= pace_pct but < 100", () => {
    const a = {
      ...ATTAINMENT,
      attainment_pct: 70,
      band: "accent" as const,
      pace_pct: 64,
    };
    render(<AttainmentRing attainment={a} isLoading={false} isError={false} />);
    expect(screen.getByText(/ahead of pace/i)).toBeInTheDocument();
  });

  it("AC-quota-3: pace line reads 'Behind pace' when attainment_pct < pace_pct", () => {
    const a = {
      ...ATTAINMENT,
      attainment_pct: 40,
      band: "behind" as const,
      pace_pct: 64,
    };
    render(<AttainmentRing attainment={a} isLoading={false} isError={false} />);
    expect(screen.getByText(/behind pace/i)).toBeInTheDocument();
  });

  it("AC-quota-2: never client-recomputes the band — a 'behind' server band still renders the danger color even at a high pct", () => {
    const a = { ...ATTAINMENT, attainment_pct: 95, band: "behind" as const };
    const { container } = render(
      <AttainmentRing attainment={a} isLoading={false} isError={false} />,
    );
    expect(container.querySelector(".text-gf-status-danger")).toBeTruthy();
  });

  it("STATE-2: shows a skeleton when isLoading", () => {
    render(
      <AttainmentRing
        attainment={undefined}
        isLoading={true}
        isError={false}
      />,
    );
    expect(screen.getByTestId("attainment-ring-skeleton")).toBeInTheDocument();
  });

  it("STATE-1: shows a distinct 'set a target' message when isTargetZero, not the generic error", () => {
    render(
      <AttainmentRing
        attainment={undefined}
        isLoading={false}
        isError={true}
        isTargetZero={true}
      />,
    );
    expect(screen.getByText(/set a target/i)).toBeInTheDocument();
    expect(screen.queryByText(/couldn't recompute/i)).not.toBeInTheDocument();
  });

  it("STATE-4: shows a distinct no-access message when isForbidden, checked before the generic error", () => {
    render(
      <AttainmentRing
        attainment={undefined}
        isLoading={false}
        isError={true}
        isForbidden={true}
      />,
    );
    expect(screen.getByText(/don't have access/i)).toBeInTheDocument();
    expect(screen.queryByText(/couldn't recompute/i)).not.toBeInTheDocument();
  });

  it("STATE-3: shows the generic honest error card otherwise", () => {
    render(
      <AttainmentRing
        attainment={undefined}
        isLoading={false}
        isError={true}
      />,
    );
    expect(screen.getByText(/couldn't recompute/i)).toBeInTheDocument();
  });
});
