import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: { GET: vi.fn(), PATCH: vi.fn() },
}));

// vitest's `clearMocks` defaults to false, and the shared `apiClient.GET`/`PATCH` mock is a single
// module-level vi.fn() reused across every test below — without this, `useOrgContacts`'s
// "no wasted call" assertion sees call history left over from earlier tests in this file.
beforeEach(() => {
  vi.clearAllMocks();
});

import { apiClient } from "../../../lib/api-client/client.js";
import {
  useOrganization,
  useOrgContacts,
  useOrgPartner,
  useSourcedDeals,
  useUpdateOrganization,
} from "./organizations.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("useOrganization", () => {
  it("reads the composite getOrganization record", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "org1", display_name: "Acme" },
      error: undefined,
      response: { status: 200 },
    });
    const { result } = renderHook(() => useOrganization("org1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/organizations/{id}",
      expect.objectContaining({ params: { path: { id: "org1" } } }),
    );
    expect(result.current.data?.display_name).toBe("Acme");
  });
});

describe("useOrgPartner (STATE-1 on 404, never STATE-3)", () => {
  it("returns null, not an error, on a 404", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { code: "not_found" },
      response: { status: 404 },
    });
    const { result } = renderHook(() => useOrgPartner("org1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toBeNull();
    expect(result.current.isError).toBe(false);
  });

  it("surfaces a genuine non-404 failure as an error (STATE-3)", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { code: "server_error" },
      response: { status: 500 },
    });
    const { result } = renderHook(() => useOrgPartner("org1"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
  });

  it("returns the partner record on 200", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "pt1", organization_id: "org1", cert_status: "certified" },
      error: undefined,
      response: { status: 200 },
    });
    const { result } = renderHook(() => useOrgPartner("org1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.cert_status).toBe("certified");
  });
});

describe("useOrgContacts (bounded N+1)", () => {
  it("fires one getPerson call per id, in parallel", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation(
      (_path: string, opts: { params: { path: { id: string } } }) =>
        Promise.resolve({
          data: {
            id: opts.params.path.id,
            full_name: `Person ${opts.params.path.id}`,
          },
          error: undefined,
          response: { status: 200 },
        }),
    );
    const { result } = renderHook(() => useOrgContacts(["p1", "p2"]), {
      wrapper,
    });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.contacts).toHaveLength(2);
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/people/{id}",
      expect.objectContaining({ params: { path: { id: "p1" } } }),
    );
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/people/{id}",
      expect.objectContaining({ params: { path: { id: "p2" } } }),
    );
  });

  it("returns [] with isLoading false when there are no ids (no wasted call)", () => {
    const { result } = renderHook(() => useOrgContacts([]), { wrapper });
    expect(result.current.contacts).toEqual([]);
    expect(result.current.isLoading).toBe(false);
    expect(apiClient.GET).not.toHaveBeenCalled();
  });
});

describe("useSourcedDeals (bounded client-side partner_org_id filter — flagged gap)", () => {
  it("filters a bounded listDeals page by partner_org_id client-side", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        data: [
          { id: "d1", partner_org_id: "org1", name: "Sourced" },
          { id: "d2", partner_org_id: "org2", name: "Not sourced" },
        ],
        page: {},
      },
      error: undefined,
      response: { status: 200 },
    });
    const { result } = renderHook(() => useSourcedDeals("org1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toHaveLength(1);
    expect(result.current.data?.[0].id).toBe("d1");
  });
});

describe("useUpdateOrganization (AC-company-12 Edit → PATCH updateOrganization)", () => {
  it("PATCHes with an If-Match version header and writes the response into the detail cache", async () => {
    (apiClient as unknown as { PATCH: ReturnType<typeof vi.fn> }).PATCH = vi
      .fn()
      .mockResolvedValueOnce({
        data: { id: "org1", display_name: "Acme", industry: "Fintech" },
        error: undefined,
        response: { status: 200 },
      });
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const wrapperWithQc = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useUpdateOrganization("org1"), {
      wrapper: wrapperWithQc,
    });
    await result.current.mutateAsync({ industry: "Fintech", version: 3 });
    expect(
      (apiClient as unknown as { PATCH: ReturnType<typeof vi.fn> }).PATCH,
    ).toHaveBeenCalledWith(
      "/organizations/{id}",
      expect.objectContaining({
        params: { path: { id: "org1" }, header: { "If-Match": "3" } },
        body: { industry: "Fintech" },
      }),
    );
    expect(qc.getQueryData(["organizations", "detail", "org1"])).toMatchObject({
      industry: "Fintech",
    });
  });
});
