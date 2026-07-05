import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../features/identity/store/authStore.js", () => ({
  useAuthStore: () => ({
    user: { id: "u1", display_name: "Admin" },
    role: "admin",
    roles: ["admin"],
    loading: false,
  }),
}));
vi.mock("../features/people/api/people.js", () => ({
  usePeople: () => ({
    data: { data: [] },
    isLoading: false,
    isError: false,
  }),
}));
vi.mock("../features/people/api/person.js", () => ({
  usePerson: () => ({
    data: undefined,
    isLoading: true,
    isError: false,
    error: null,
    refetch: vi.fn(),
  }),
  useArchivePerson: () => ({ mutate: vi.fn(), isPending: false }),
  useRestorePerson: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("../features/organizations/api/organizations.js", () => ({
  useOrganizations: () => ({
    data: { data: [] },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useArchiveOrganization: () => ({ mutate: vi.fn(), isPending: false }),
  useRestoreOrganization: () => ({ mutate: vi.fn(), isPending: false }),
}));

import App from "./App.js";

function renderApp(initialEntry: string) {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("App routes", () => {
  it("mounts PeoplePage at /people", () => {
    renderApp("/people");
    expect(
      screen.getByRole("heading", { name: /contacts we actually know/i }),
    ).toBeInTheDocument();
  });

  it("mounts PersonDetailPage at /people/:id", () => {
    renderApp("/people/abc");
    expect(screen.getByTestId("person-detail-loading")).toBeInTheDocument();
  });

  it("mounts ShellPlaceholderPage for rail routes without a real feature", () => {
    renderApp("/reports");
    expect(screen.getByText(/reports — coming soon/i)).toBeInTheDocument();
  });

  it("mounts CompaniesPage at /companies", () => {
    renderApp("/companies");
    expect(
      screen.getByRole("heading", { name: /companies/i }),
    ).toBeInTheDocument();
  });
});
