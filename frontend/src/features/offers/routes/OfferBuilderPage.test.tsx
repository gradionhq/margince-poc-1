import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { canMutateOffer, OfferBuilderPage } from "./OfferBuilderPage.js";

const refetchOffer = vi.fn();
const refetchDealOffers = vi.fn();
const refetchDeal = vi.fn();
const createLineItemMutate = vi.fn();
const updateLineItemMutate = vi.fn();
const deleteLineItemMutate = vi.fn();

let mockRole: string | null = "admin";
let mockUserId = "u1";
const mockDeal = {
  id: "d1",
  name: "Acme Renewal",
  workspace_id: "w1",
};
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
  valid_until: "2026-08-01T00:00:00Z",
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
let mockDealLoading = false;
let mockLineItems = [
  {
    id: "l1",
    offer_id: "o2",
    description: "Committed line",
    unit: "ea",
    quantity: 2,
    unit_price_minor: 1000,
    discount_pct: 0,
    tax_rate: 20,
    source: "ui",
    captured_by: "human:u1",
    evidence: null,
    price_grounded: true,
    position: 1,
  },
  {
    id: "l2",
    offer_id: "o2",
    description: "Staged AI line",
    unit: "ea",
    quantity: 1,
    unit_price_minor: 2500,
    discount_pct: 0,
    tax_rate: 20,
    source: "ai",
    captured_by: "agent:assistant",
    evidence: { snippet: "Add a premium support line", source_id: "src1" },
    price_grounded: false,
    position: 2,
  },
];

vi.mock("../../identity/store/authStore.js", () => ({
  useAuthStore: () => ({
    user: { id: mockUserId },
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
  useRegenerateOffer: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  useCreateLineItem: () => ({
    mutate: createLineItemMutate,
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  useUpdateLineItem: () => ({
    mutate: updateLineItemMutate,
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  useDeleteLineItem: () => ({
    mutate: deleteLineItemMutate,
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  useRenderOffer: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  useSendOffer: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  useDealOffers: () => ({
    data: mockDealOffersError ? undefined : { data: mockDealOffers, page: {} },
    isLoading: mockDealOffersLoading,
    isError: mockDealOffersError,
    error: mockDealOffersError ? { status: 500 } : null,
    refetch: refetchDealOffers,
  }),
  useOfferLineItems: () => ({
    data: mockLineItems,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  }),
}));

vi.mock("../../deals/api/deals.js", () => ({
  useDeal: () => ({
    data: mockDealLoading ? undefined : mockDeal,
    isLoading: mockDealLoading,
    isError: false,
    error: null,
    refetch: refetchDeal,
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
    mockDealLoading = false;
    mockUserId = "u1";
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
      valid_until: "2026-08-01T00:00:00Z",
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
    mockLineItems = [
      {
        id: "l1",
        offer_id: "o2",
        description: "Committed line",
        unit: "ea",
        quantity: 2,
        unit_price_minor: 1000,
        discount_pct: 0,
        tax_rate: 20,
        source: "ui",
        captured_by: "human:u1",
        evidence: null,
        price_grounded: true,
        position: 1,
      },
      {
        id: "l2",
        offer_id: "o2",
        description: "Staged AI line",
        unit: "ea",
        quantity: 1,
        unit_price_minor: 2500,
        discount_pct: 0,
        tax_rate: 20,
        source: "ai",
        captured_by: "agent:assistant",
        evidence: { snippet: "Add a premium support line", source_id: "src1" },
        price_grounded: false,
        position: 2,
      },
    ];
    createLineItemMutate.mockClear();
    updateLineItemMutate.mockClear();
    deleteLineItemMutate.mockClear();
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
    expect(
      screen.getByRole("button", { name: /try again/i }),
    ).toBeInTheDocument();
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

    expect(
      screen.getByRole("heading", { name: /off-1 v2/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to deal/i })).toHaveAttribute(
      "href",
      "/deals/d1",
    );
    expect(screen.getByText("draft")).toBeInTheDocument();
    expect(screen.getByTestId("offer-versions-bar")).toBeInTheDocument();
    expect(screen.getByTestId("locked-revision-icon")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /v1/i })).toBeInTheDocument();
  });

  it("composes the full offer builder and shows the send card on drafts", () => {
    renderPage();

    expect(
      screen.getByRole("heading", { name: /regenerate/i }),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/Committed line/i)).toHaveLength(2);
    expect(
      screen.getByRole("heading", { name: /staged ai lines/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Explain this total/i)).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: /angebot/i }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("send-card")).toBeInTheDocument();
    expect(
      screen.getByText(/your own click here is the approval/i),
    ).toBeInTheDocument();
  });

  it("hides every edit affordance and the send card once the offer is sent", () => {
    mockOffer = { ...mockOffer, status: "sent" };
    mockDealOffers = [{ ...mockOffer, id: "o2", status: "sent" }];
    renderPage();

    expect(screen.queryByTestId("send-card")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /add line/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/this revision is locked/i)).toBeInTheDocument();
  });

  it("navigates to a locked revision when its pill is clicked", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("link", { name: /v1/i }));
    expect(
      screen.getByRole("heading", { name: /off-1 v2/i }),
    ).toBeInTheDocument();
  });

  it("wires the composed editor and staged panel to the real line item mutations", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /add line/i }));
    expect(createLineItemMutate).toHaveBeenCalledWith({
      position: 2,
      description: "New line",
      quantity: 1,
      unit_price_minor: 0,
      discount_pct: 0,
      tax_rate: 0,
      source: "ui",
      captured_by: "human:u1",
    });

    const qtyInput = screen.getByLabelText("qty Committed line");
    await user.clear(qtyInput);
    await user.type(qtyInput, "3");
    await user.tab();
    expect(updateLineItemMutate).toHaveBeenCalledWith({
      lineId: "l1",
      patch: {
        quantity: 3,
        unit_price_minor: 1000,
        discount_pct: 0,
        tax_rate: 20,
      },
    });

    await user.click(screen.getByRole("button", { name: /delete/i }));
    expect(deleteLineItemMutate).toHaveBeenCalledWith({ lineId: "l1" });

    await user.click(screen.getByRole("button", { name: /accept/i }));
    expect(updateLineItemMutate).toHaveBeenCalledWith({
      lineId: "l2",
      patch: {
        captured_by: "human:u1",
        source: "ui",
      },
    });

    await user.click(screen.getByRole("button", { name: /dismiss/i }));
    expect(deleteLineItemMutate).toHaveBeenCalledWith({ lineId: "l2" });
  });
});
