import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { OrganizationHierarchyRollup } from "../api/records.js";
import { RollupExplainBox } from "./RollupExplainBox.js";

const rollup: OrganizationHierarchyRollup = {
  root_id: "org-1",
  scope: "tree",
  weighted_pipeline: { amount_minor: 38_500_00, currency: "EUR" },
  closed_won: { amount_minor: 12_000_00, currency: "EUR" },
  activity_count_30d: 7,
  aggregated_account_count: 4,
  restricted_excluded: [{ id: "r1", display_name: "Restricted Corp" }],
  computed_at: "2026-07-09T00:00:00Z",
};

describe("RollupExplainBox", () => {
  it("is collapsed by default — no formula text visible", () => {
    render(
      <RollupExplainBox
        rollup={rollup}
        selfFigure={{ amount_minor: 10_000_00, currency: "EUR" }}
        childrenSumFigure={{ amount_minor: 28_500_00, currency: "EUR" }}
      />,
    );
    expect(screen.queryByText(/roll-up\(node\)/i)).not.toBeInTheDocument();
  });

  it("clicking 'Explain this roll-up' expands the formula", async () => {
    render(
      <RollupExplainBox
        rollup={rollup}
        selfFigure={{ amount_minor: 10_000_00, currency: "EUR" }}
        childrenSumFigure={{ amount_minor: 28_500_00, currency: "EUR" }}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: /explain this roll-up/i }),
    );
    expect(
      screen.getByText(/roll-up\(node\) = self\(node\) \+ Σ roll-up\(child\)/i),
    ).toBeInTheDocument();
  });

  it("shows self and children-sum figures when expanded", async () => {
    render(
      <RollupExplainBox
        rollup={rollup}
        selfFigure={{ amount_minor: 10_000_00, currency: "EUR" }}
        childrenSumFigure={{ amount_minor: 28_500_00, currency: "EUR" }}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: /explain this roll-up/i }),
    );
    expect(screen.getByText("Self")).toBeInTheDocument();
    expect(screen.getByText("Children sum")).toBeInTheDocument();
  });

  it("names each restricted_excluded node when expanded", async () => {
    render(
      <RollupExplainBox
        rollup={rollup}
        selfFigure={{ amount_minor: 10_000_00, currency: "EUR" }}
        childrenSumFigure={{ amount_minor: 28_500_00, currency: "EUR" }}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: /explain this roll-up/i }),
    );
    expect(screen.getByText(/Restricted Corp/)).toBeInTheDocument();
  });

  it("clicking again collapses the formula", async () => {
    render(
      <RollupExplainBox
        rollup={rollup}
        selfFigure={{ amount_minor: 10_000_00, currency: "EUR" }}
        childrenSumFigure={{ amount_minor: 28_500_00, currency: "EUR" }}
      />,
    );
    const btn = screen.getByRole("button", { name: /explain this roll-up/i });
    await userEvent.click(btn);
    await userEvent.click(btn);
    expect(screen.queryByText(/roll-up\(node\)/i)).not.toBeInTheDocument();
  });
});
