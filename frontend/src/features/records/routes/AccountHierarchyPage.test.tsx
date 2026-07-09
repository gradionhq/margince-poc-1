import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: {
    GET: vi.fn(),
    PATCH: vi.fn(),
    POST: vi.fn(),
    DELETE: vi.fn(),
  },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import { recordsKeys } from "../api/records.js";
import { AccountHierarchyPage } from "./AccountHierarchyPage.js";

const ROOT_ID = "org-root";

const treeRollup = {
  root_id: ROOT_ID,
  scope: "tree",
  weighted_pipeline: { amount_minor: 38_500_00, currency: "EUR" },
  closed_won: { amount_minor: 12_000_00, currency: "EUR" },
  activity_count_30d: 7,
  aggregated_account_count: 3,
  restricted_excluded: [],
  computed_at: "2026-07-09T00:00:00Z",
};

const selfRollup = {
  ...treeRollup,
  scope: "self",
  weighted_pipeline: { amount_minor: 10_000_00, currency: "EUR" },
  closed_won: { amount_minor: 5_000_00, currency: "EUR" },
  aggregated_account_count: 1,
};

const rootOrg = {
  id: ROOT_ID,
  workspace_id: "ws-1",
  display_name: "Root Corp",
  source: "test",
  captured_by: "human:test",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  parent_org_id: null,
  domains: [
    {
      id: "d1",
      organization_id: ROOT_ID,
      domain: "rootcorp.com",
      is_primary: true,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      archived_at: null,
    },
  ],
  version: 1,
};

const childOrg = {
  id: "org-child",
  workspace_id: "ws-1",
  display_name: "Child Corp",
  source: "test",
  captured_by: "human:test",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  parent_org_id: ROOT_ID,
  domains: [],
  version: 1,
};

function makeClient(overrides: Record<string, unknown> = {}): QueryClient {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  qc.setQueryData(recordsKeys.rollup(ROOT_ID, "tree"), treeRollup);
  qc.setQueryData(recordsKeys.rollup(ROOT_ID, "self"), selfRollup);
  qc.setQueryData(recordsKeys.treeOrgs(), [rootOrg, childOrg]);
  qc.setQueryData(["organizations", "detail", ROOT_ID], rootOrg);
  Object.entries(overrides).forEach(([key, val]) => {
    // key is JSON-encoded query key array
    qc.setQueryData(JSON.parse(key), val);
  });
  return qc;
}

function wrapper(qc: QueryClient) {
  return function Wrapper({ children: _children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={[`/companies/${ROOT_ID}/hierarchy`]}>
          <Routes>
            <Route
              path="/companies/:id/hierarchy"
              element={<AccountHierarchyPage />}
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
  };
}

describe("AccountHierarchyPage", () => {
  it("STATE-1: renders skeletons while loading, no fabricated numbers", () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    // Don't seed any data — queries stay in loading state with no mock
    (apiClient.GET as ReturnType<typeof vi.fn>).mockReturnValue(
      new Promise(() => {}),
    );
    render(<AccountHierarchyPage />, { wrapper: wrapper(qc) });
    expect(
      screen.getByTestId("rollup-tiles-band-skeleton"),
    ).toBeInTheDocument();
  });

  it("STATE-2: an empty tree (no children, no restricted) renders honest 'no sub-accounts' state", () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    qc.setQueryData(recordsKeys.rollup(ROOT_ID, "tree"), {
      ...treeRollup,
      aggregated_account_count: 1,
      restricted_excluded: [],
    });
    qc.setQueryData(recordsKeys.rollup(ROOT_ID, "self"), selfRollup);
    qc.setQueryData(recordsKeys.treeOrgs(), [rootOrg]);
    qc.setQueryData(["organizations", "detail", ROOT_ID], rootOrg);
    render(<AccountHierarchyPage />, { wrapper: wrapper(qc) });
    expect(screen.getByText(/no sub-accounts/i)).toBeInTheDocument();
  });

  it("STATE-3: rollup query failing renders an error state", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    // Seed treeOrgs and detail but NOT the rollup — RollupTilesBand shows error when isError=true
    qc.setQueryData(recordsKeys.treeOrgs(), [rootOrg]);
    qc.setQueryData(["organizations", "detail", ROOT_ID], rootOrg);
    // The rollup query will fail because we return an error from apiClient
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: undefined,
      error: new Error("Server error"),
    });
    render(<AccountHierarchyPage />, { wrapper: wrapper(qc) });
    // Wait for the error state to render after the async query fails
    await waitFor(() =>
      expect(screen.getByText(/failed to load/i)).toBeInTheDocument(),
    );
  });

  it("renders tree scope tiles and tree nodes from seeded data", () => {
    const qc = makeClient();
    render(<AccountHierarchyPage />, { wrapper: wrapper(qc) });
    expect(screen.getByText("Root Corp")).toBeInTheDocument();
    expect(screen.getByText("Child Corp")).toBeInTheDocument();
    expect(screen.getByText(/aggregated over 3 accounts/i)).toBeInTheDocument();
  });

  it("AC-3: switching ScopeToggle updates what scope's rollup is displayed", async () => {
    const qc = makeClient();
    render(<AccountHierarchyPage />, { wrapper: wrapper(qc) });
    // Initially tree scope — shows tree aggregated count
    expect(screen.getByText(/aggregated over 3 accounts/i)).toBeInTheDocument();
    // Switch to self scope
    await userEvent.click(
      screen.getByRole("radio", { name: /this account only/i }),
    );
    // self rollup has aggregated_account_count: 1
    expect(screen.getByText(/aggregated over 1 account/i)).toBeInTheDocument();
  });

  it("AC-4: toggling a parent row's twist collapses/expands its children", async () => {
    const qc = makeClient();
    render(<AccountHierarchyPage />, { wrapper: wrapper(qc) });
    // Root is a parent — child is visible (root expanded by default)
    expect(screen.getByText("Child Corp")).toBeInTheDocument();
    // Find the toggle button on root's row
    const rootRow = screen.getByText("Root Corp").closest("tr");
    const toggle = rootRow?.querySelector("button");
    if (toggle) {
      await userEvent.click(toggle);
      // After collapse, child should not be visible
      expect(screen.queryByText("Child Corp")).not.toBeInTheDocument();
      // Click again to expand
      await userEvent.click(toggle);
      expect(screen.getByText("Child Corp")).toBeInTheDocument();
    }
  });

  it("AC-7: dismissing a suggested edge card removes it locally with no mutation call", async () => {
    const orphanOrg = {
      id: "org-orphan",
      workspace_id: "ws-1",
      display_name: "Orphan Corp",
      source: "test",
      captured_by: "human:test",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      parent_org_id: null,
      domains: [
        {
          id: "d2",
          organization_id: "org-orphan",
          domain: "rootcorp.com",
          is_primary: true,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
          archived_at: null,
        },
      ],
      version: 1,
    };
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    qc.setQueryData(recordsKeys.rollup(ROOT_ID, "tree"), treeRollup);
    qc.setQueryData(recordsKeys.rollup(ROOT_ID, "self"), selfRollup);
    qc.setQueryData(recordsKeys.treeOrgs(), [rootOrg, childOrg, orphanOrg]);
    qc.setQueryData(["organizations", "detail", ROOT_ID], rootOrg);

    render(<AccountHierarchyPage />, { wrapper: wrapper(qc) });

    expect(screen.getByText(/Orphan Corp/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /dismiss/i }));
    expect(screen.queryByText(/Orphan Corp/)).not.toBeInTheDocument();
    // No PATCH call was made
    expect(apiClient.PATCH).not.toHaveBeenCalled();
  });
});
