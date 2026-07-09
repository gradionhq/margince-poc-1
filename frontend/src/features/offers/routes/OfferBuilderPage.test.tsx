import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { OfferBuilderPage, canMutateOffer } from "./OfferBuilderPage.js";

const refetchOffer = vi.fn();
const refetchDealOffers = vi.fn();

let mockRole: string | null = "admin";
let mockOffer = {
  id: "o2",
  workspace_id: "w1",
  deal_id: "d1",
  offer_number: "OFF-1",
  revision: 2,
  status: "draft",
  currency: "EUR",
  source: "test",
  captured_by: "human:test",
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  line_items: [],
  net_minor: 0,
  tax_minor: 0,
  gross_minor: 0,
};
let mockDealOffers = [
  mockOffer,
  {
    ...mockOffer,
    id: "o1",
    revision: 1,
    status: "superseded",
  },
];
let mockOfferError = false;
let mockOfferErrorStatus: number | null = null;
let mockDealOffersError = false;
let mockOfferLoading = false;
let mockDealOffersLoading = false;

vi.mock("../../identity/store/authStore.js", () => ({
  useAuthStore: () => ({
    user: { id: "u1" },
    role: mockRole,
    roles: [mockRole].filter(Boolean),
    loading: false,
  }),
}));

vi.mock("../api/offers.js", () => ({
  useOffer: () => ({
    data: mockOfferError ? undefined : mockOffer,
    isLoading: mockOfferLoading,
    isError: mockOfferError,
    error: mockOfferError ? { status: mockOfferErrorStatus ?? 500 } : null,
    refetch: refetchOffer,
  }),
  useDealOffers: () => ({
    data: mockDealOffersError ? undefined : { data: mockDealOffers, page: {} },
    isLoading: mockDealOffersLoading,
    isError: mockDealOffersError,
    error: mockDealOffersError ? { status: 500 } : null,
    refetch: refetchDealOffers,
  }),
}));

function renderPage(entry = "/deals/d1/offers/o2") {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route
            path="/deals/:id/offers/:offerId"
            element={<OfferBuilderPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("canMutateOffer", () => {
  it("uses the shared allowlist and draft-only gate", () => {
    expect(canMutateOffer("admin", { status: "draft" })).toBe(true);
    expect(canMutateOffer("rep", { status: "draft" })).toBe(true);
    expect(canMutateOffer("manager", { status: "draft" })).toBe(true);
    expect(canMutateOffer("read_only", { status: "draft" })).toBe(false);
    expect(canMutateOffer("ops", { status: "draft" })).toBe(false);
    expect(canMutateOffer("admin", { status: "sent" })).toBe(false);
  });
});

describe("OfferBuilderPage", () => {
  beforeEach(() => {
    mockRole = "admin";
    mockOfferError = false;
    mockOfferErrorStatus = null;
    mockDealOffersError = false;
    mockOfferLoading = false;
    mockDealOffersLoading = false;
    mockOffer = {
      id: "o2",
      workspace_id: "w1",
      deal_id: "d1",
      offer_number: "OFF-1",
      revision: 2,
      status: "draft",
      currency: "EUR",
      source: "test",
      captured_by: "human:test",
      created_at: "2026-07-01T00:00:00Z",
      updated_at: "2026-07-01T00:00:00Z",
      line_items: [],
      net_minor: 0,
      tax_minor: 0,
      gross_minor: 0,
    };
    mockDealOffers = [
      mockOffer,
      { ...mockOffer, id: "o1", revision: 1, status: "superseded" },
    ];
  });

  it("renders the loading skeleton immediately", () => {
    mockOfferLoading = true;
    renderPage();
    expect(screen.getByTestId("offer-builder-skeleton")).toBeInTheDocument();
  });

  it("renders an honest error card on load failure", () => {
    mockOfferError = true;
    mockOfferErrorStatus = 500;
    mockRole = "admin";
    renderPage();
    expect(screen.getByTestId("offer-builder-error-card")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
  });

  it("renders a permission card for ops", () => {
    mockRole = "ops";
    mockOfferError = true;
    mockOfferErrorStatus = 403;
    renderPage();
    expect(
      screen.getByText(/you don't have permission to view this offer/i),
    ).toBeInTheDocument();
  });

  it("renders header, parent link, status pill, and locked versions", () => {
    renderPage();

    expect(screen.getByRole("heading", { name: /off-1 v2/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to deal/i })).toHaveAttribute(
      "href",
      "/deals/d1",
    );
    expect(screen.getByText("draft")).toBeInTheDocument();
    expect(screen.getByTestId("offer-versions-bar")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /v1/i })).toBeInTheDocument();
  });

  it("navigates to a locked revision when its pill is clicked", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("link", { name: /v1/i }));
    expect(screen.getByRole("heading", { name: /off-1 v2/i })).toBeInTheDocument();
  });
});
