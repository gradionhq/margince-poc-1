import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: {
    GET: vi.fn(),
    POST: vi.fn(),
    DELETE: vi.fn(),
    PATCH: vi.fn(),
  },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import {
  HierarchyRollupForbiddenError,
  useAccountTreeOrgs,
  useOrganizationHierarchyRollup,
} from "./records.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("useOrganizationHierarchyRollup", () => {
  it("calls GET /organizations/{id}/hierarchy-rollup with path + query.scope", async () => {
    const fixture = {
      root_id: "org-1",
      scope: "tree" as const,
      weighted_pipeline: { amount_minor: 100000, currency: "EUR" },
      closed_won: { amount_minor: 50000, currency: "EUR" },
      activity_count_30d: 5,
      aggregated_account_count: 3,
      restricted_excluded: [],
      computed_at: "2026-07-09T00:00:00Z",
    };
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: fixture,
      error: undefined,
    });

    const { result } = renderHook(
      () => useOrganizationHierarchyRollup("org-1", "tree"),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(apiClient.GET).toHaveBeenCalledWith(
      "/organizations/{id}/hierarchy-rollup",
      expect.objectContaining({
        params: {
          path: { id: "org-1" },
          query: { scope: "tree" },
        },
      }),
    );
    // Returns the roll-up unmodified (never re-derives/sums it — the whole point of AC-1/AC-3).
    expect(result.current.data?.weighted_pipeline.amount_minor).toBe(100000);
    expect(result.current.data?.aggregated_account_count).toBe(3);
  });

  it("is disabled when rootId is undefined", () => {
    const { result } = renderHook(
      () => useOrganizationHierarchyRollup(undefined, "tree"),
      { wrapper },
    );
    expect(result.current.fetchStatus).toBe("idle");
  });

  it("STATE-4: surfaces a distinct HierarchyRollupForbiddenError on a 403, not a generic error", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { code: "forbidden" },
      response: { status: 403 },
    });

    const { result } = renderHook(
      () => useOrganizationHierarchyRollup("org-1", "tree"),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(HierarchyRollupForbiddenError);
  });

  it("a non-403 failure still surfaces as a generic error, not HierarchyRollupForbiddenError", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { code: "server_error" },
      response: { status: 500 },
    });

    const { result } = renderHook(
      () => useOrganizationHierarchyRollup("org-1", "tree"),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).not.toBeInstanceOf(
      HierarchyRollupForbiddenError,
    );
  });
});

describe("useAccountTreeOrgs", () => {
  it("calls GET /organizations with limit: 200 and returns data.data", async () => {
    const orgs = [
      { id: "o1", display_name: "Alpha", parent_org_id: null },
      { id: "o2", display_name: "Beta", parent_org_id: "o1" },
    ];
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { data: orgs, page: { has_more: false } },
      error: undefined,
    });

    const { result } = renderHook(() => useAccountTreeOrgs(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(apiClient.GET).toHaveBeenCalledWith(
      "/organizations",
      expect.objectContaining({
        params: { query: { limit: 200 } },
      }),
    );
    expect(result.current.data).toHaveLength(2);
    expect(result.current.data?.[0].id).toBe("o1");
  });
});
