import { render, screen } from "@testing-library/react";
import { fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { LineItemRow } from "./LineItemRow.js";

const line: OfferLineItem = {
  id: "li-1",
  workspace_id: "w1",
  offer_id: "o1",
  position: 1,
  description: "Consulting",
  unit: "hour",
  quantity: 2,
  unit_price_minor: 1999,
  discount_pct: 12.5,
  tax_rate: 19,
  source: "test",
  captured_by: "human:test",
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  evidence: null,
  price_grounded: true,
};

describe("LineItemRow", () => {
  it("renders read-only values and the computed line total", () => {
    render(
      <LineItemRow
        line={line}
        canMutateOffer={false}
        onUpdate={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    expect(screen.getByText("Consulting")).toBeInTheDocument();
    expect(screen.getByText("3,498")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /delete/i })).not.toBeInTheDocument();
  });

  it("lets a mutable row recompute on keystroke and patch on blur", async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();

    render(
      <LineItemRow
        line={line}
        canMutateOffer={true}
        onUpdate={onUpdate}
        onDelete={vi.fn()}
      />,
    );

    const qty = screen.getByRole("spinbutton", { name: /qty consulting/i });
    await user.clear(qty);
    await user.type(qty, "3");

    expect(screen.getByText("5,247")).toBeInTheDocument();

    fireEvent.blur(qty);
    expect(onUpdate).toHaveBeenCalledWith("li-1", {
      quantity: 3,
      unit_price_minor: 1999,
      discount_pct: 12.5,
      tax_rate: 19,
    });
  });
});
