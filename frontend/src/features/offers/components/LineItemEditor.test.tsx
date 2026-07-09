import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { OfferLineItem } from "../../../lib/api-client/generated/index.js";
import { LineItemEditor } from "./LineItemEditor.js";

const stagedLine: OfferLineItem = {
  id: "li-2",
  workspace_id: "w1",
  offer_id: "o1",
  position: 2,
  description: "AI proposed",
  unit: "unit",
  quantity: 1,
  unit_price_minor: 1200,
  discount_pct: 0,
  tax_rate: 10,
  source: "agent:test",
  captured_by: "agent:test",
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  evidence: { snippet: "scope line", source_id: "src-1" },
  price_grounded: true,
};

const humanLine: OfferLineItem = {
  ...stagedLine,
  id: "li-1",
  position: 1,
  description: "Discovery",
  captured_by: "human:test",
  source: "manual",
  evidence: null,
};

describe("LineItemEditor", () => {
  it("renders only non-staged lines and reconciles the footer total", () => {
    render(
      <LineItemEditor
        lines={[humanLine, stagedLine]}
        stagedLineIds={new Set(["li-2"])}
        canMutateOffer={true}
        onCreate={vi.fn()}
        onUpdate={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    expect(screen.getByText("Discovery")).toBeInTheDocument();
    expect(screen.queryByText("AI proposed")).not.toBeInTheDocument();
    expect(
      screen.getByText(/excludes 1 staged ai-proposed line/i),
    ).toBeInTheDocument();
    expect(screen.getByText("1,200")).toBeInTheDocument();
  });

  it("shows an honest empty state when there are no lines", () => {
    render(
      <LineItemEditor
        lines={[]}
        stagedLineIds={new Set()}
        canMutateOffer={false}
        onCreate={vi.fn()}
        onUpdate={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    expect(screen.getByText(/no line items yet/i)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /add line/i }),
    ).not.toBeInTheDocument();
  });

  it("still offers an Add line affordance on a zero-line draft when the user can mutate the offer", async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    render(
      <LineItemEditor
        lines={[]}
        stagedLineIds={new Set()}
        canMutateOffer={true}
        onCreate={onCreate}
        onUpdate={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    expect(screen.getByText(/no line items yet/i)).toBeInTheDocument();
    const addLineButton = screen.getByRole("button", { name: /add line/i });
    expect(addLineButton).toBeInTheDocument();

    await user.click(addLineButton);
    expect(onCreate).toHaveBeenCalledTimes(1);
  });
});
