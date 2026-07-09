import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { ExplainTotalPanel } from "./ExplainTotalPanel.js";

const humanLine: OfferLineItem = {
  id: "line-human",
  workspace_id: "ws-1",
  offer_id: "offer-1",
  position: 1,
  description: "Discovery workshop",
  unit: "hour",
  quantity: 2,
  unit_price_minor: 2500,
  discount_pct: 10,
  tax_rate: 20,
  source: "ui",
  captured_by: "human:user-1",
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  evidence: null,
  price_grounded: true,
};

const stagedPricedLine: OfferLineItem = {
  id: "line-staged",
  workspace_id: "ws-1",
  offer_id: "offer-1",
  position: 2,
  description: "AI suggested scope",
  unit: "hour",
  quantity: 1,
  unit_price_minor: 5000,
  discount_pct: 0,
  tax_rate: 20,
  source: "agent:regen",
  captured_by: "agent:regen",
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  evidence: { snippet: "Add integration support", source_id: "src-1" },
  price_grounded: true,
};

const humanUnpricedLine: OfferLineItem = {
  ...humanLine,
  id: "line-human-unpriced",
  description: "No price yet",
  unit_price_minor: 0,
  price_grounded: false,
};

describe("ExplainTotalPanel", () => {
  it("shows the per-line formula and excludes staged and unpriced lines from the explanation", async () => {
    const user = userEvent.setup();
    render(
      <ExplainTotalPanel
        currency="USD"
        lines={[humanLine, stagedPricedLine, humanUnpricedLine]}
        grossMinor={99999}
      />,
    );

    const toggle = screen.getByRole("button", { name: /explain this total/i });
    expect(toggle).toBeInTheDocument();
    await user.click(toggle);
    expect(screen.getByText("Discovery workshop")).toBeInTheDocument();
    expect(
      screen.getByText(/2 × 2500 × \(1 - 10%\) = 4500/),
    ).toBeInTheDocument();
    expect(screen.getByText(/4500 × 20% = 900/)).toBeInTheDocument();
    expect(screen.queryByText("AI suggested scope")).not.toBeInTheDocument();
    expect(
      screen.getByText(
        /1 staged AI-proposed line\(s\) and 1 unpriced line\(s\) are excluded/i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /persisted record still carries 1 pending AI line\(s\)/i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/amounts in USD minor units \(cents\); ISO 4217/i),
    ).toBeInTheDocument();
  });

  it("marks the gross caption as computed server-side once no staged lines remain", async () => {
    const user = userEvent.setup();
    render(
      <ExplainTotalPanel
        currency="EUR"
        lines={[humanLine]}
        grossMinor={5400}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: /explain this total/i }),
    );

    expect(
      screen.getByText(/gross minor units are computed server-side/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/gross minor units: 5400/i)).toBeInTheDocument();
    expect(
      screen.getByText(/persisted record total: 5400/i),
    ).toBeInTheDocument();
  });
});
