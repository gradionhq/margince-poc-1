import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { RegenerateBanner } from "./RegenerateBanner.js";

const navigateSpy = vi.fn();
const mutateAsync = vi.fn();

vi.mock("react-router-dom", async () => {
  const actual =
    await vi.importActual<typeof import("react-router-dom")>(
      "react-router-dom",
    );
  return {
    ...actual,
    useNavigate: () => navigateSpy,
  };
});

vi.mock("../api/offers.js", () => ({
  useRegenerateOffer: () => ({
    mutateAsync,
    isPending: false,
  }),
}));

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={qc}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
}

describe("RegenerateBanner", () => {
  it("hides itself for draft offers even for mutate-capable roles", () => {
    const { container } = render(
      <RegenerateBanner
        dealId="deal-1"
        offer={{
          id: "offer-1",
          offer_number: "OFF-1",
          revision: 1,
          status: "draft",
          currency: "EUR",
        }}
        userRole="admin"
        onRegenerated={vi.fn()}
      />,
      { wrapper },
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("shows itself for sent offers when the role can mutate", () => {
    render(
      <RegenerateBanner
        dealId="deal-1"
        offer={{
          id: "offer-1",
          offer_number: "OFF-1",
          revision: 1,
          status: "sent",
          currency: "EUR",
        }}
        userRole="rep"
        onRegenerated={vi.fn()}
      />,
      { wrapper },
    );

    expect(
      screen.getByRole("button", { name: /regenerate/i }),
    ).toBeInTheDocument();
  });

  it("navigates to the regenerated draft and uses the mutation response for the banner", async () => {
    mutateAsync.mockResolvedValueOnce({
      id: "offer-2",
      deal_id: "deal-1",
      offer_number: "OFF-1",
      revision: 2,
      status: "draft",
      currency: "EUR",
      ai_generated: true,
      ai_disclosure: "AI disclosure from the server",
      diff_from_previous: {
        added: [
          {
            id: "li-3",
            description: "Added line",
            quantity: 1,
            unit_price_minor: 1200,
            discount_pct: 0,
            tax_rate: 20,
            source: "agent:regen",
            captured_by: "agent:regen",
            evidence: { snippet: "draft scope" },
          },
        ],
        removed: [
          {
            id: "li-4",
            description: "Removed line",
            quantity: 2,
            unit_price_minor: 500,
            discount_pct: 0,
            tax_rate: 10,
            source: "human:seed",
            captured_by: "human:seed",
            evidence: null,
          },
        ],
        changed: [
          {
            before: {
              id: "li-5",
              description: "Before line",
              quantity: 1,
              unit_price_minor: 1000,
              discount_pct: 5,
              tax_rate: 20,
              source: "human:seed",
              captured_by: "human:seed",
              evidence: null,
            },
            after: {
              id: "li-5",
              description: "After line",
              quantity: 3,
              unit_price_minor: 1500,
              discount_pct: 0,
              tax_rate: 20,
              source: "agent:regen",
              captured_by: "agent:regen",
              evidence: { snippet: "scope update" },
            },
          },
        ],
      },
    });

    render(
      <RegenerateBanner
        dealId="deal-1"
        offer={{
          id: "offer-1",
          offer_number: "OFF-1",
          revision: 1,
          status: "sent",
          currency: "EUR",
        }}
        userRole="admin"
        onRegenerated={vi.fn()}
      />,
      { wrapper },
    );

    fireEvent.click(screen.getByRole("button", { name: /regenerate/i }));

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({ offerId: "offer-1" }),
    );
    await waitFor(() =>
      expect(navigateSpy).toHaveBeenCalledWith("/deals/deal-1/offers/offer-2"),
    );
    expect(
      screen.getByText("v1 → v2 — 1 added, 1 removed, 1 changed"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("AI disclosure from the server"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /view full diff/i }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /view full diff/i }));
    expect(screen.getByText("Added line")).toBeInTheDocument();
    expect(screen.getByText("Before line → After line")).toBeInTheDocument();
  });

  it("omits AI disclosure and the diff table for a plain regenerate", async () => {
    mutateAsync.mockResolvedValueOnce({
      id: "offer-2",
      deal_id: "deal-1",
      offer_number: "OFF-1",
      revision: 2,
      status: "draft",
      currency: "EUR",
      ai_generated: false,
      ai_disclosure: null,
      diff_from_previous: null,
    });

    render(
      <RegenerateBanner
        dealId="deal-1"
        offer={{
          id: "offer-1",
          offer_number: "OFF-1",
          revision: 1,
          status: "sent",
          currency: "EUR",
        }}
        userRole="admin"
        onRegenerated={vi.fn()}
      />,
      { wrapper },
    );

    fireEvent.click(screen.getByRole("button", { name: /regenerate/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());
    expect(screen.queryByText(/AI disclosure from the server/i)).toBeNull();
    expect(
      screen.queryByRole("button", { name: /view full diff/i }),
    ).toBeNull();
  });
});
