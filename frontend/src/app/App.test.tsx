import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../shared/ui/ToastContainer.js", () => ({
  ToastContainer: () => <div>Toasts</div>,
}));

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
vi.mock("../features/custom-fields/api/customFields.js", () => ({
  useCustomFields: () => ({
    data: {
      data: [
        {
          id: "field-1",
          workspace_id: "ws-1",
          object: "deal",
          label: "Test Field",
          slug: "test_field",
          type: "text",
          status: "active",
          column_name: "cf_test_field",
          created_by: "user-1",
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      ],
      page: { total: 1 },
    },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useCreateCustomField: () => ({ mutate: vi.fn(), isPending: false }),
  useRenameCustomField: () => ({ mutate: vi.fn(), isPending: false }),
  useRetireCustomField: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateCustomFieldOptions: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("../features/custom-fields/api/members.js", () => ({
  useMembers: () => ({
    data: { data: [] },
    isLoading: false,
    isError: false,
  }),
}));
vi.mock("../features/records/api/quotas.js", () => ({
  useQuota: () => ({
    data: undefined,
    isLoading: true,
    isError: false,
    error: null,
  }),
  useQuotaAttainment: () => ({
    data: undefined,
    isLoading: true,
    isError: false,
    error: null,
  }),
  useTeamRollup: () => ({ reps: [], isLoading: false, isError: false }),
  QuotaForbiddenError: class extends Error {},
  QuotaAttainmentForbiddenError: class extends Error {},
  QuotaAttainmentTargetZeroError: class extends Error {},
  QuotaAttainmentComputationFailedError: class extends Error {},
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
      screen.getByRole("heading", { level: 2, name: /companies/i }),
    ).toBeInTheDocument();
  });

  it("mounts CustomFieldsAdminPage at /admin/custom-fields", () => {
    renderApp("/admin/custom-fields");
    expect(
      screen.getByRole("heading", { name: /custom fields/i }),
    ).toBeInTheDocument();
  });

  it("mounts QuotaPage at /quotas/:id", () => {
    renderApp("/quotas/q1");
    expect(screen.getByText(/quota & attainment/i)).toBeInTheDocument();
  });
});
