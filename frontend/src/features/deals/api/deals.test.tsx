import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: {
    GET: vi.fn(),
  },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import {
  useDeal,
  useDeals,
  useDefaultPipeline,
  usePipelineRollup,
  useStages,
} from "./deals.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
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
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/deals/d1",
      expect.anything(),
    );
  });
});
