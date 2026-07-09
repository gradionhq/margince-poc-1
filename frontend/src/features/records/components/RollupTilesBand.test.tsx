import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { OrganizationHierarchyRollup } from "../api/records.js";
import { RollupTilesBand } from "./RollupTilesBand.js";

const rollup: OrganizationHierarchyRollup = {
  root_id: "org-1",
  scope: "tree",
  weighted_pipeline: { amount_minor: 38_500_00, currency: "EUR" },
  closed_won: { amount_minor: 12_000_00, currency: "EUR" },
  activity_count_30d: 7,
  aggregated_account_count: 4,
  restricted_excluded: [],
  computed_at: "2026-07-09T00:00:00Z",
};

describe("RollupTilesBand", () => {
  it("renders weighted-pipeline, closed-won, and 30-day-activity tiles from the rollup", () => {
    render(
      <RollupTilesBand
        rollup={rollup}
        isLoading={false}
        isError={false}
        depth={2}
        nodeCount={4}
      />,
    );
    expect(screen.getByText(/weighted pipeline/i)).toBeInTheDocument();
    expect(screen.getByText(/closed.won.*fy26/i)).toBeInTheDocument();
    expect(screen.getByText(/30.day activity/i)).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
  });

  it("shows 'aggregated over N accounts' using aggregated_account_count", () => {
    render(
      <RollupTilesBand
        rollup={rollup}
        isLoading={false}
        isError={false}
        depth={2}
        nodeCount={4}
      />,
    );
    expect(screen.getByText(/aggregated over 4 accounts/i)).toBeInTheDocument();
  });

  it("shows the budget-bar line with depth, nodeCount, and % of P11 budget", () => {
    render(
      <RollupTilesBand
        rollup={rollup}
        isLoading={false}
        isError={false}
        depth={3}
        nodeCount={100}
      />,
    );
    // 100 / 200 * 100 = 50%
    expect(screen.getByText(/depth 3/i)).toBeInTheDocument();
    expect(screen.getByText(/100 nodes/i)).toBeInTheDocument();
    expect(screen.getByText(/50% of P11 budget/i)).toBeInTheDocument();
  });

  it("shows a loading skeleton when isLoading", () => {
    render(
      <RollupTilesBand
        rollup={undefined}
        isLoading={true}
        isError={false}
        depth={0}
        nodeCount={0}
      />,
    );
    expect(
      screen.getByTestId("rollup-tiles-band-skeleton"),
    ).toBeInTheDocument();
  });

  it("shows an honest error card when isError", () => {
    render(
      <RollupTilesBand
        rollup={undefined}
        isLoading={false}
        isError={true}
        depth={0}
        nodeCount={0}
      />,
    );
    expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
  });

  it("STATE-4: shows a distinct no-permission message when isForbidden, not the generic error text", () => {
    render(
      <RollupTilesBand
        rollup={undefined}
        isLoading={false}
        isError={true}
        isForbidden={true}
        depth={0}
        nodeCount={0}
      />,
    );
    expect(
      screen.getByText(/you don't have access to this account's roll-up/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/failed to load/i)).not.toBeInTheDocument();
  });

  it("renders the EUR · ISO-4217 · integer minor-units label", () => {
    render(
      <RollupTilesBand
        rollup={rollup}
        isLoading={false}
        isError={false}
        depth={2}
        nodeCount={4}
      />,
    );
    expect(
      screen.getByText(/EUR · ISO-4217 · integer minor-units/),
    ).toBeInTheDocument();
  });
});
