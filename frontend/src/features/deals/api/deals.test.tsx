import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: {
    GET: vi.fn(),
    POST: vi.fn(),
  },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import {
  dealsKeys,
  useAdvanceDeal,
  useCreateDeal,
  useDeal,
  useDeals,
  useDefaultPipeline,
  useOpenDealsForOrg,
  usePipelineRollup,
  useRecentActivityCount,
  useStages,
} from "./deals.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("deals read API", () => {
  it("useDefaultPipeline finds the is_default: true pipeline", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        data: [
          { id: "p1", is_default: false, name: "Old" },
          { id: "p2", is_default: true, name: "Default" },
        ],
        page: {},
      },
      error: undefined,
    });
    const { result } = renderHook(() => useDefaultPipeline(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.id).toBe("p2");
  });

  it("useStages fetches ordered stages for a pipeline", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { data: [{ id: "s1", position: 0 }], page: {} },
      error: undefined,
    });
    const { result } = renderHook(() => useStages("p2"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/stages",
      expect.objectContaining({
        params: { query: { pipeline_id: "p2" } },
      }),
    );
    expect(result.current.data).toHaveLength(1);
  });

  it("useDeals filters by pipeline/stage/status", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { data: [], page: {} },
      error: undefined,
    });
    const { result } = renderHook(
      () => useDeals({ pipelineId: "p2", status: "open" }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/deals",
      expect.objectContaining({
        params: {
          query: { pipeline_id: "p2", stage_id: undefined, status: "open" },
        },
      }),
    );
  });

  it("usePipelineRollup reads the roll-up, never sums client-side", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        pipeline_id: "p2",
        unweighted_minor: 100,
        weighted_minor: 40,
        base_currency: "EUR",
        as_of_date: "2026-07-04",
        by_stage: [],
        breakdown: [],
      },
      error: undefined,
    });
    const { result } = renderHook(() => usePipelineRollup("p2"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/pipelines/p2/rollup",
      expect.anything(),
    );
    expect(result.current.data?.weighted_minor).toBe(40);
  });

  it("useDeal reads the deal-360 composite", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "d1", name: "Acme deal" },
      error: undefined,
    });
    const { result } = renderHook(() => useDeal("d1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith("/deals/d1", expect.anything());
  });
});

describe("useAdvanceDeal (optimistic mutation)", () => {
  it("patches the cache in onMutate before the network resolves", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    qc.setQueryData(dealsKeys.list("p1", undefined, "open"), {
      data: [
        { id: "d1", stage_id: "s0", stage_entered_at: "2020-01-01T00:00:00Z" },
      ],
      page: {},
    });
    let resolvePost!: (v: unknown) => void;
    (apiClient.POST as ReturnType<typeof vi.fn>).mockReturnValueOnce(
      new Promise((r) => {
        resolvePost = r;
      }),
    );
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useAdvanceDeal("p1"), {
      wrapper: localWrapper,
    });

    act(() => {
      result.current.mutate({ dealId: "d1", toStageId: "s1" });
    });

    // Optimistic patch is synchronous — check before the network resolves.
    const optimistic = qc.getQueryData<{
      data: Array<{ id: string; stage_id: string }>;
    }>(dealsKeys.list("p1", undefined, "open"));
    expect(optimistic?.data[0].stage_id).toBe("s1");

    resolvePost({ data: { id: "d1", stage_id: "s1" }, error: undefined });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
  });

  it("rolls back the cache onError and surfaces the server-named cause", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    qc.setQueryData(dealsKeys.list("p1", undefined, "open"), {
      data: [{ id: "d1", stage_id: "s0" }],
      page: {},
    });
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { code: "validation_error", detail: "stage not in pipeline" },
    });
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useAdvanceDeal("p1"), {
      wrapper: localWrapper,
    });

    await act(async () => {
      try {
        await result.current.mutateAsync({ dealId: "d1", toStageId: "s1" });
      } catch {
        // expected — asserted via result.current.isError below
      }
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    const rolledBack = qc.getQueryData<{
      data: Array<{ id: string; stage_id: string }>;
    }>(dealsKeys.list("p1", undefined, "open"));
    expect(rolledBack?.data[0].stage_id).toBe("s0");
  });
});

describe("useCreateDeal", () => {
  it("posts a CreateDealRequest and returns the created deal", async () => {
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "d9", name: "New Acme deal" },
      error: undefined,
    });
    const qc = new QueryClient();
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useCreateDeal(), {
      wrapper: localWrapper,
    });
    const created = await result.current.mutateAsync({
      name: "New Acme deal",
      pipeline_id: "p1",
      stage_id: "s0",
      organization_id: "o1",
      source: "manual",
      captured_by: "human:u1",
    });
    expect(created.id).toBe("d9");
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/deals",
      expect.objectContaining({
        body: expect.objectContaining({ organization_id: "o1" }),
      }),
    );
  });
});

describe("useOpenDealsForOrg", () => {
  it("filters listDeals by organization_id + status=open (duplicate-deal check)", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { data: [{ id: "d1" }], page: {} },
      error: undefined,
    });
    const qc = new QueryClient();
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useOpenDealsForOrg("o1"), {
      wrapper: localWrapper,
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/deals",
      expect.objectContaining({
        params: { query: { organization_id: "o1", status: "open" } },
      }),
    );
    expect(result.current.data?.data).toHaveLength(1);
  });
});

describe("useRecentActivityCount", () => {
  it("counts the returned page of organization-linked activities honestly (no fabricated total)", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { data: [{ id: "a1" }, { id: "a2" }], page: { has_more: false } },
      error: undefined,
    });
    const qc = new QueryClient();
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useRecentActivityCount("o1"), {
      wrapper: localWrapper,
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/activities",
      expect.objectContaining({
        params: {
          query: { entity_type: "organization", entity_id: "o1", limit: 10 },
        },
      }),
    );
    expect(result.current.data).toBe(2);
  });
});
