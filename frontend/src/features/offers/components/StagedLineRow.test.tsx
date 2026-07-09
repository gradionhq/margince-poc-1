import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { StagedLineRow } from "./StagedLineRow.js";

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

const unpricedLine: OfferLineItem = {
  ...stagedLine,
  id: "li-2",
  description: "Unpriced AI line",
  unit_price_minor: 0,
  price_grounded: false,
  evidence: { snippet: "scope line", source_id: "src-2" },
};

describe("StagedLineRow", () => {
  it("accepts, edits, and dismisses a priced staged line", async () => {
    const user = userEvent.setup();
    const onAccept = vi.fn().mockResolvedValue(undefined);
    const onDismiss = vi.fn().mockResolvedValue(undefined);

    render(
      <StagedLineRow
        line={stagedLine}
        canMutateOffer
        currentUserId="u1"
        onAccept={onAccept}
        onDismiss={onDismiss}
      />,
    );

    expect(screen.getByText(/ai-proposed/i)).toBeInTheDocument();
    expect(screen.getByText("scope line")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /accept/i }));
    expect(onAccept).toHaveBeenCalledWith("li-1", {
      captured_by: "human:u1",
      source: "ui",
    });
    expect(
      screen.getByText(/accepted — now part of your draft/i),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /edit/i }));
    await user.clear(
      screen.getByRole("spinbutton", { name: /qty ai proposed line/i }),
    );
    await user.type(
      screen.getByRole("spinbutton", { name: /qty ai proposed line/i }),
      "3",
    );
    await user.click(screen.getByRole("button", { name: /save edits/i }));
    expect(onAccept).toHaveBeenCalledWith("li-1", {
      quantity: 3,
      unit_price_minor: 2500,
      discount_pct: 0,
      tax_rate: 20,
      captured_by: "human:u1",
      source: "ui",
    });

    await user.click(screen.getByRole("button", { name: /dismiss/i }));
    expect(onDismiss).toHaveBeenCalledWith("li-1");
    expect(
      screen.getByText(/dismissed — removed from this draft/i),
    ).toBeInTheDocument();
  });

  it("requires a positive price before accepting an unpriced line", async () => {
    const user = userEvent.setup();
    const onAccept = vi.fn();

    render(
      <StagedLineRow
        line={unpricedLine}
        canMutateOffer
        currentUserId="u1"
        onAccept={onAccept}
        onDismiss={vi.fn()}
      />,
    );

    expect(
      screen.getByText(/we won't guess a number for this line/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /accept/i })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: /edit/i }));
    await user.type(
      screen.getByRole("spinbutton", { name: /price for unpriced ai line/i }),
      "1250",
    );
    expect(screen.getByRole("button", { name: /accept/i })).toBeEnabled();
  });

  it("renders read-only when the offer cannot be mutated", () => {
    render(
      <StagedLineRow
        line={stagedLine}
        canMutateOffer={false}
        currentUserId="u1"
        onAccept={vi.fn()}
        onDismiss={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: /accept/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /dismiss/i }),
    ).not.toBeInTheDocument();
  });
});
