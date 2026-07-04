import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

const baseOrgData = {
  id: "org1",
  display_name: "Acme Corp",
  industry: "Software",
  size_band: "51-200",
  domains: [{ domain: "acme.com", is_primary: true }],
  address: { city: "Berlin", country: "DE" },
  org_strength: null,
  deals: [],
  relationships: [],
  activities: [],
  contact_count: 0,
  open_deal_count: 0,
  version: 3,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  source: "manual",
  captured_by: "human:u1",
};

let mockPartner: unknown = null;
let mockMutate = vi.fn((_patch: unknown, opts?: { onSuccess?: () => void }) =>
  opts?.onSuccess?.(),
);

vi.mock("../api/organizations.js", () => ({
  useOrganization: () => ({
    data: baseOrgData,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useOrgPartner: () => ({
    data: mockPartner,
    isLoading: false,
    isError: false,
  }),
  useOrgContacts: () => ({ contacts: [], isLoading: false }),
  useSourcedDeals: () => ({ data: [], isLoading: false, isError: false }),
  useUpdateOrganization: () => ({ mutate: mockMutate, isPending: false }),
}));

import { CompanyDetailPage } from "./CompanyDetailPage.js";

function renderPage() {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={["/companies/org1"]}>
        <Routes>
          <Route path="/companies/:id" element={<CompanyDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CompanyDetailPage", () => {
  beforeEach(() => {
    mockPartner = null;
    mockMutate = vi.fn((_patch: unknown, opts?: { onSuccess?: () => void }) =>
      opts?.onSuccess?.(),
    );
  });

  it("renders the header card with name, industry, website, staff, location", () => {
    renderPage();
    expect(screen.getByText("Acme Corp")).toBeInTheDocument();
    expect(screen.getByText("Software")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /acme\.com/i })).toHaveAttribute(
      "href",
      "https://acme.com",
    );
    expect(screen.getByText(/51-200/)).toBeInTheDocument();
    expect(screen.getByText("Berlin, DE")).toBeInTheDocument();
  });

  it("renders the no-signal org strength card", () => {
    renderPage();
    expect(screen.getByText(/no signal yet/i)).toBeInTheDocument();
  });

  it("renders honest empty states for PeopleRail and DealRail when there are none", () => {
    renderPage();
    expect(screen.getByText("No known contacts yet.")).toBeInTheDocument();
    expect(
      screen.getByText("No open or won deals for this org."),
    ).toBeInTheDocument();
  });

  it("renders honest empty states for ActivityCard, AccountSignalCard, and QuickFactsRail", () => {
    renderPage();
    expect(screen.getByText(/no activity yet/i)).toBeInTheDocument();
    expect(
      screen.getByText(/no account signal to flag right now/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/owner/i)).toBeInTheDocument();
  });

  it("renders no partner panel when the org is not a partner (STATE-1, not an error)", () => {
    mockPartner = null;
    renderPage();
    expect(screen.queryByText("Cert status")).not.toBeInTheDocument();
  });

  it("renders the partner panel when the org IS a partner", () => {
    mockPartner = {
      id: "pt1",
      organization_id: "org1",
      cert_status: "certified",
      partner_role: "consulting",
    };
    renderPage();
    expect(screen.getByText("Cert status")).toBeInTheDocument();
    expect(screen.getByText(/certified/i)).toBeInTheDocument();
  });

  it("shows Summarize this account disabled with an explanatory title", () => {
    renderPage();
    const summarizeBtn = screen.getByRole("button", {
      name: /summarize this account/i,
    });
    expect(summarizeBtn).toBeDisabled();
    expect(summarizeBtn).toHaveAttribute(
      "title",
      "Account summaries ship in a later chapter",
    );
  });

  it("clicking Edit opens EditOrgModal, saving a changed field marks it typed-by-you (AC-company-12)", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));
    const industryInput = screen.getByDisplayValue("Software");
    fireEvent.change(industryInput, { target: { value: "Fintech" } });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(screen.getByTitle("Typed by you this session")).toBeInTheDocument();
  });
});
