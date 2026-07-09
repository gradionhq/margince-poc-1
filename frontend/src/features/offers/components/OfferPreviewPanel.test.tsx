import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Offer } from "../../../lib/api-client/generated/index.js";
import { OfferPreviewPanel } from "./OfferPreviewPanel.js";

vi.mock("../api/offers.js", () => ({
  useRenderOffer: () => ({
    mutateAsync: vi.fn().mockResolvedValue({
      id: "offer-1",
      pdf_asset_ref: "https://example.com/offer.pdf",
    }),
    isPending: false,
  }),
}));

const offer: Offer = {
  id: "offer-1",
  workspace_id: "w1",
  deal_id: "deal-1",
  template_id: "tpl-1",
  offer_number: "OFF-001",
  revision: 3,
  status: "draft",
  currency: "EUR",
  title: "Unused server title",
  valid_until: "2026-08-01T00:00:00Z",
  net_minor: 123456,
  tax_minor: 23456,
  gross_minor: 146912,
  ai_generated: false,
  ai_disclosure: null,
  diff_from_previous: null,
  pdf_asset_ref: null,
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
};

describe("OfferPreviewPanel", () => {
  it("swaps labels, date formatting, and at least one money string when toggled", async () => {
    render(
      <OfferPreviewPanel
        dealName="Acme Deal"
        offer={offer}
        canMutateOffer
      />,
    );

    expect(screen.getByText("Angebot")).toBeInTheDocument();
    expect(screen.getByText("Angebotsnummer")).toBeInTheDocument();
    expect(screen.getByText("01.08.26")).toBeInTheDocument();
    expect(
      screen.getAllByText(
        (content) => content.includes("1.234,56") && content.includes("€"),
      ).length,
    ).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "EN" }));

    expect(screen.getByText("Offer")).toBeInTheDocument();
    expect(screen.getByText("Offer number")).toBeInTheDocument();
    expect(screen.getByText("8/1/26")).toBeInTheDocument();
    expect(screen.getAllByText("€1,234.56").length).toBeGreaterThan(0);
  });

  it("calls renderOffer from the Generate PDF action and then shows a view link", async () => {
    render(
      <OfferPreviewPanel
        dealName="Acme Deal"
        offer={offer}
        canMutateOffer
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /pdf erzeugen/i }));

    await waitFor(() =>
      expect(screen.getByRole("link", { name: /pdf ansehen/i })).toHaveAttribute(
        "href",
        "https://example.com/offer.pdf",
      ),
    );
  });

  it("hides the generate action when the role cannot mutate the offer", () => {
    render(
      <OfferPreviewPanel
        dealName="Acme Deal"
        offer={offer}
        canMutateOffer={false}
      />,
    );

    expect(
      screen.queryByRole("button", { name: /pdf erzeugen/i }),
    ).not.toBeInTheDocument();
  });
});
