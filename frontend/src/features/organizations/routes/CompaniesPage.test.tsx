import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

let mockArchiveMutate = vi.fn(
  (_vars: void, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.(),
);

vi.mock("../api/organizations.js", () => ({
  useOrganizations: () => ({
    data: {
      data: [
        {
          id: "o1",
          display_name: "Acme Inc",
          industry: "Software",
          contact_count: 1,
          open_deal_count: 0,
          org_strength: null,
        },
      ],
    },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useArchiveOrganization: () => ({
    mutate: mockArchiveMutate,
    isPending: false,
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
  beforeEach(() => {
    mockArchiveMutate = vi.fn(
      (_vars: void, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.(),
    );
  });

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
  it("navigates to /companies/:id when a row is clicked (row click-through)", () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={["/companies"]}>
          <Routes>
            <Route path="/companies" element={<CompaniesPage />} />
            <Route
              path="/companies/:id"
              element={<div>Company detail o1</div>}
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText("Acme Inc"));
    expect(screen.getByText("Company detail o1")).toBeInTheDocument();
  });
  it("opens archive confirm from the row menu and shows a success toast on confirm", async () => {
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: /row actions/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: /archive/i }));

    expect(
      screen.getByText(/acme inc will be removed from the default list/i),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /archive/i }));

    await waitFor(() =>
      expect(screen.getByText(/acme inc archived/i)).toBeInTheDocument(),
    );
  });
});
