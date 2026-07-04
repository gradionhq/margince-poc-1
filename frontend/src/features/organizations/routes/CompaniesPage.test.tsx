import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/organizations.js", () => ({
  useOrganizations: () => ({
    data: { data: [] },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("../../identity/store/authStore.js", () => ({
  useAuthStore: () => ({
    user: { id: "u1", display_name: "Admin" },
    role: "admin",
  }),
}));

import { CompaniesPage } from "./CompaniesPage.js";

function renderPage() {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <CompaniesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CompaniesPage", () => {
  it("renders the Companies section label", () => {
    renderPage();
    expect(
      screen.getByRole("heading", { name: /companies/i }),
    ).toBeInTheDocument();
  });
  it("renders the Strength sort control", () => {
    renderPage();
    expect(
      screen.getByRole("button", { name: /strength/i }),
    ).toBeInTheDocument();
  });
  it("renders the Filter button", () => {
    renderPage();
    expect(screen.getByRole("button", { name: /filter/i })).toBeInTheDocument();
  });
  it("renders the search input", () => {
    renderPage();
    expect(screen.getByPlaceholderText(/search/i)).toBeInTheDocument();
  });
  it("renders the New button flagged as rare path", () => {
    renderPage();
    expect(screen.getByRole("button", { name: /new/i })).toBeInTheDocument();
  });
  it("has no capture banner (PO-N-PILOT)", () => {
    renderPage();
    expect(screen.queryByText(/capture/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/banner/i)).not.toBeInTheDocument();
  });
});
