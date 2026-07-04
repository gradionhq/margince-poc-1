import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/organizations.js", () => ({
  useOrganization: () => ({
    data: {
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
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      source: "manual",
      captured_by: "human:u1",
    },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useOrgPartner: () => ({ data: null, isLoading: false, isError: false }),
  useOrgContacts: () => ({ contacts: [], isLoading: false }),
  useSourcedDeals: () => ({ data: [], isLoading: false, isError: false }),
  useUpdateOrganization: () => ({ mutate: vi.fn(), isPending: false }),
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
});
