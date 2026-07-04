import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: {
    GET: vi.fn(),
    POST: vi.fn(),
    PATCH: vi.fn(),
  },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import {
  useMergePerson,
  useOrganizationName,
  usePerson,
  usePersonDeals,
  usePersonStrengthBreakdown,
  useUpdatePerson,
} from "./person.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("person API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("usePerson fetches the composite record by id", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "p1", full_name: "Alice" },
      error: undefined,
    });
    const { result } = renderHook(() => usePerson("p1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/people/{id}",
      expect.objectContaining({ params: { path: { id: "p1" } } }),
    );
    expect(result.current.data?.full_name).toBe("Alice");
  });

  it("usePersonStrengthBreakdown stays disabled until enabled: true", () => {
    const { result } = renderHook(
      () => usePersonStrengthBreakdown("p1", false),
      { wrapper },
    );
    expect(result.current.fetchStatus).toBe("idle");
    expect(apiClient.GET).not.toHaveBeenCalled();
  });

  it("usePersonStrengthBreakdown fetches evidence when enabled", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        person_id: "p1",
        score: 42,
        recency: 0.5,
        frequency: 0.4,
        reciprocity: 0.6,
        contributing_activities: [],
      },
      error: undefined,
    });
    const { result } = renderHook(
      () => usePersonStrengthBreakdown("p1", true),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/people/{id}/strength-breakdown",
      expect.objectContaining({ params: { path: { id: "p1" } } }),
    );
  });

  it("usePersonDeals filters listDeals by person_id (DEAL-EXT-2)", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { data: [{ id: "d1", name: "Deal 1" }], page: {} },
      error: undefined,
    });
    const { result } = renderHook(() => usePersonDeals("p1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/deals",
      expect.objectContaining({ params: { query: { person_id: "p1" } } }),
    );
    expect(result.current.data).toHaveLength(1);
  });

  it("useOrganizationName is disabled without an id and fetches display_name with one", async () => {
    const { result: disabled } = renderHook(
      () => useOrganizationName(undefined),
      {
        wrapper,
      },
    );
    expect(disabled.current.fetchStatus).toBe("idle");

    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "o1", display_name: "Acme Corp" },
      error: undefined,
    });
    const { result } = renderHook(() => useOrganizationName("o1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toBe("Acme Corp");
  });

  it("useMergePerson posts { target_id } and surfaces the raw error object on failure", async () => {
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { status: 409, code: "version_skew", detail: "concurrent merge" },
    });
    const { result } = renderHook(() => useMergePerson("p1"), { wrapper });
    result.current.mutate({ targetId: "p2" });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/people/{id}/merge",
      expect.objectContaining({
        params: { path: { id: "p1" } },
        body: { target_id: "p2" },
      }),
    );
    expect((result.current.error as { code?: string }).code).toBe(
      "version_skew",
    );
  });

  it("useUpdatePerson sends If-Match when a version is supplied", async () => {
    (apiClient.PATCH as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "p1", full_name: "New Name" },
      error: undefined,
    });
    const { result } = renderHook(() => useUpdatePerson("p1"), { wrapper });
    result.current.mutate({ body: { full_name: "New Name" }, ifMatch: "3" });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.PATCH).toHaveBeenCalledWith(
      "/people/{id}",
      expect.objectContaining({
        params: { path: { id: "p1" }, header: { "If-Match": "3" } },
        body: { full_name: "New Name" },
      }),
    );
  });
});
