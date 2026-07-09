import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: { GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn(), PATCH: vi.fn() },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import {
  QuotaAttainmentComputationFailedError,
  QuotaAttainmentForbiddenError,
  QuotaAttainmentTargetZeroError,
  QuotaForbiddenError,
  parseGermanIntegerEuros,
  quotasKeys,
  shouldRetryQuotaAttainment,
  useContributingDealDetails,
  useQuota,
  useQuotaAttainment,
  useTeamRollup,
  useUpdateQuotaTarget,
} from "./quotas.js";

beforeEach(() => vi.clearAllMocks());

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

const FIXTURE_ATTAINMENT = {
  quota_id: "q1",
  closed_won_minor: 31387200,
  target_minor: 28000000,
  currency: "EUR",
  attainment_pct: 112.1,
  gap_minor: 3387200,
  pace_pct: 92.4,
  band: "met" as const,
  as_of_date: "2026-07-09",
  contributing_deals: [],
};

describe("quotasKeys", () => {
  it("creates stable cache keys", () => {
    expect(quotasKeys.detail("q1")).toEqual(["quotas", "detail", "q1"]);
    expect(quotasKeys.attainment("q1")).toEqual(["quotas", "attainment", "q1"]);
    expect(quotasKeys.list()).toEqual(["quotas", "list"]);
  });
});

describe("useQuota", () => {
  it("calls GET /quotas/{id} with the path id", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "q1", target_minor: 28000000 },
      error: undefined,
    });

    const { result } = renderHook(() => useQuota("q1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(apiClient.GET).toHaveBeenCalledWith(
      "/quotas/{id}",
      expect.objectContaining({
        params: { path: { id: "q1" } },
      }),
    );
  });

  it("surfaces QuotaForbiddenError on a 403", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { code: "forbidden" },
      response: { status: 403 },
    });

    const { result } = renderHook(() => useQuota("q1"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(QuotaForbiddenError);
  });
});

describe("useQuotaAttainment", () => {
  it("calls GET /quotas/{id}/attainment with the path id", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: FIXTURE_ATTAINMENT,
      error: undefined,
      response: { status: 200 },
    });

    const { result } = renderHook(() => useQuotaAttainment("q1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(apiClient.GET).toHaveBeenCalledWith(
      "/quotas/{id}/attainment",
      expect.objectContaining({
        params: { path: { id: "q1" } },
      }),
    );
    expect(result.current.data?.quota_id).toBe("q1");
  });

  it("maps 403 to QuotaAttainmentForbiddenError", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { code: "forbidden" },
      response: { status: 403 },
    });

    const { result } = renderHook(() => useQuotaAttainment("q1"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(QuotaAttainmentForbiddenError);
  });

  it("maps 422 attainment_target_zero to QuotaAttainmentTargetZeroError", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { code: "attainment_target_zero" },
      response: { status: 422 },
    });

    const { result } = renderHook(() => useQuotaAttainment("q1"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(QuotaAttainmentTargetZeroError);
  });

  it("maps other 422s to QuotaAttainmentComputationFailedError", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: { code: "attainment_computation_failed" },
      response: { status: 422 },
    });

    const { result } = renderHook(() => useQuotaAttainment("q1"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(
      QuotaAttainmentComputationFailedError,
    );
  });
});

describe("shouldRetryQuotaAttainment", () => {
  it("never retries sentinel errors", () => {
    expect(shouldRetryQuotaAttainment(0, new QuotaAttainmentForbiddenError())).toBe(false);
    expect(shouldRetryQuotaAttainment(0, new QuotaAttainmentTargetZeroError())).toBe(false);
    expect(
      shouldRetryQuotaAttainment(0, new QuotaAttainmentComputationFailedError()),
    ).toBe(false);
  });

  it("bounds retries for any other error", () => {
    expect(shouldRetryQuotaAttainment(0, new Error("blip"))).toBe(true);
    expect(shouldRetryQuotaAttainment(2, new Error("blip"))).toBe(false);
  });
});

describe("useUpdateQuotaTarget", () => {
  it("PATCHes /quotas/{id} with target_minor + If-Match: version", async () => {
    (apiClient.PATCH as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "q1", target_minor: 28000000, version: 4 },
      error: undefined,
    });
    const { result } = renderHook(() => useUpdateQuotaTarget("q1"), { wrapper });
    result.current.mutate({ targetMinor: 28000000, version: 3 });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.PATCH).toHaveBeenCalledWith(
      "/quotas/{id}",
      expect.objectContaining({
        params: { path: { id: "q1" }, header: { "If-Match": "3" } },
        body: { target_minor: 28000000 },
      }),
    );
  });
});

describe("parseGermanIntegerEuros", () => {
  it("parses a German-grouped integer euro string to minor units", () => {
    expect(parseGermanIntegerEuros("280.000")).toBe(28000000);
    expect(parseGermanIntegerEuros("1.234")).toBe(123400);
  });

  it("returns 0 for empty/garbled input (the caller's refusal signal)", () => {
    expect(parseGermanIntegerEuros("")).toBe(0);
    expect(parseGermanIntegerEuros("abc")).toBe(0);
  });
});

describe("useContributingDealDetails", () => {
  it("fetches one GET /deals/{id} per id, in parallel", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: {
        id: "d1",
        name: "BAER Pharma - Packaging QA",
        closed_at: "2026-08-14T00:00:00Z",
      },
      error: undefined,
    });
    const { result } = renderHook(() => useContributingDealDetails(["d1", "d2"]), {
      wrapper,
    });
    await waitFor(() => expect(result.current.every((r) => !r.isLoading)).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledTimes(2);
    expect(result.current[0].data?.name).toBe("BAER Pharma - Packaging QA");
  });
});

describe("useTeamRollup", () => {
  it("keeps only sibling quotas sharing the current quota's exact period, excludes the current id", async () => {
    const current = {
      id: "q1",
      owner_id: "u1",
      team_id: null,
      period_start: "2026-07-01",
      period_end: "2026-09-30",
      target_minor: 28000000,
      currency: "EUR",
      version: 3,
    };
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation((path: string) => {
      if (path === "/quotas") {
        return Promise.resolve({
          data: {
            data: [
              current,
              {
                id: "q2",
                owner_id: "u2",
                team_id: null,
                period_start: "2026-07-01",
                period_end: "2026-09-30",
                target_minor: 27000000,
                currency: "EUR",
                version: 1,
              },
              {
                id: "q3",
                owner_id: "u3",
                team_id: null,
                period_start: "2026-04-01",
                period_end: "2026-06-30",
                target_minor: 24000000,
                currency: "EUR",
                version: 1,
              },
            ],
          },
          error: undefined,
        });
      }
      return Promise.resolve({
        data: FIXTURE_ATTAINMENT,
        error: undefined,
        response: { status: 200 },
      });
    });
    const { result } = renderHook(
      () => useTeamRollup(current, FIXTURE_ATTAINMENT),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    const ids = result.current.reps.map((r) => r.quota.id);
    expect(ids).toContain("q2");
    expect(ids).not.toContain("q3");
    expect(ids).not.toContain("q1");
  });
});
