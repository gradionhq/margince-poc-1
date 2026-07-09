import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { StagedLinesPanel } from "./StagedLinesPanel.js";

const stagedLine: OfferLineItem = {
  id: "li-1",
  workspace_id: "w1",
  offer_id: "o1",
  position: 1,
  description: "AI proposed line",
  unit: "hour",
  quantity: 2,
  unit_price_minor: 2500,
  discount_pct: 0,
  tax_rate: 20,
  source: "agent:regen",
  captured_by: "agent:regen",
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  evidence: { snippet: "scope line", source_id: "src-1" },
  price_grounded: true,
};

const humanLine: OfferLineItem = {
  ...stagedLine,
  id: "li-2",
  description: "Human line",
  captured_by: "human:test",
  evidence: null,
};

describe("StagedLinesPanel", () => {
  it("renders nothing when there are no staged lines", () => {
    const { container } = render(
      <StagedLinesPanel
        lines={[humanLine]}
        canMutateOffer
        currentUserId="u1"
        onAccept={vi.fn()}
        onDismiss={vi.fn()}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders staged rows and disclosure when staged lines exist", () => {
    render(
      <StagedLinesPanel
        lines={[humanLine, stagedLine]}
        canMutateOffer
        currentUserId="u1"
        onAccept={vi.fn()}
        onDismiss={vi.fn()}
      />,
    );

    expect(screen.getByText(/ai-proposed content/i)).toBeInTheDocument();
    expect(screen.getByText("AI proposed line")).toBeInTheDocument();
    expect(screen.getByText(/scope line/i)).toBeInTheDocument();
  });

  it("keeps staged content read-only when the offer cannot be mutated", () => {
    render(
      <StagedLinesPanel
        lines={[stagedLine]}
        canMutateOffer={false}
        currentUserId="u1"
        onAccept={vi.fn()}
        onDismiss={vi.fn()}
      />,
    );

    expect(screen.getByText("AI proposed line")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /accept/i })).not.toBeInTheDocument();
  });
});
