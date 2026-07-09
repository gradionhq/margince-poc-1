import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
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
import type {
  Offer,
  OfferLineItemListResponse,
} from "../../../lib/api-client/generated/index.js";
import {
  offersKeys,
  useCreateLineItem,
  useCreateOffer,
  useDealOffers,
  useDeleteLineItem,
  useOffer,
  useOfferLineItems,
  useRegenerateOffer,
  useRenderOffer,
  useSendOffer,
  useUpdateLineItem,
} from "./offers.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("offers read API", () => {
  it("useDealOffers fetches deal-scoped offers", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { data: [], page: {} },
      error: undefined,
    });
    const { result } = renderHook(() => useDealOffers("d1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/deals/d1/offers",
      expect.objectContaining({
        params: { path: { id: "d1" } },
      }),
    );
  });

  it("useOffer reads one offer by id", async () => {
    const offer: Offer = {
      id: "o1",
      workspace_id: "w1",
      deal_id: "d1",
      offer_number: "OFF-1",
      revision: 1,
      status: "draft",
      currency: "EUR",
      source: "test",
      captured_by: "human:test",
      version: 1,
      ai_generated: false,
      created_at: "2026-07-01T00:00:00Z",
      updated_at: "2026-07-01T00:00:00Z",
      line_items: [],
      net_minor: 0,
      tax_minor: 0,
      gross_minor: 0,
    };
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: offer,
      error: undefined,
    });
    const { result } = renderHook(() => useOffer("o1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.id).toBe("o1");
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/offers/o1",
      expect.objectContaining({ params: { path: { id: "o1" } } }),
    );
  });

  it("useOfferLineItems returns line items sorted by position", async () => {
    const lineItems: OfferLineItemListResponse = {
      data: [
        {
          id: "li2",
          workspace_id: "w1",
          offer_id: "o1",
          position: 2,
          description: "Second",
          unit: "unit",
          quantity: 1,
          unit_price_minor: 200,
          discount_pct: 0,
          tax_rate: 0,
          source: "test",
          captured_by: "human:test",
          created_at: "2026-07-01T00:00:00Z",
          updated_at: "2026-07-01T00:00:00Z",
          evidence: null,
          price_grounded: true,
        },
        {
          id: "li1",
          workspace_id: "w1",
          offer_id: "o1",
          position: 1,
          description: "First",
          unit: "unit",
          quantity: 1,
          unit_price_minor: 100,
          discount_pct: 0,
          tax_rate: 0,
          source: "test",
          captured_by: "human:test",
          created_at: "2026-07-01T00:00:00Z",
          updated_at: "2026-07-01T00:00:00Z",
          evidence: null,
          price_grounded: true,
        },
      ],
      page: { has_more: false },
    };
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: lineItems,
      error: undefined,
    });
    const { result } = renderHook(() => useOfferLineItems("o1"), {
      wrapper,
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.map((line) => line.id)).toEqual(["li1", "li2"]);
  });
});

describe("offers write API", () => {
  it("useCreateOffer sends a fresh Idempotency-Key and invalidates deal offers on success", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const uuidSpy = vi
      .spyOn(globalThis.crypto, "randomUUID")
      .mockReturnValueOnce("00000000-0000-0000-0000-000000000001")
      .mockReturnValueOnce("00000000-0000-0000-0000-000000000002");
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "o2",
        workspace_id: "w1",
        deal_id: "d1",
        offer_number: "OFF-2",
        revision: 1,
        status: "draft",
        currency: "EUR",
        source: "test",
        captured_by: "human:test",
        created_at: "2026-07-01T00:00:00Z",
        updated_at: "2026-07-01T00:00:00Z",
        line_items: [],
        net_minor: 0,
        tax_minor: 0,
        gross_minor: 0,
      },
      error: undefined,
    });
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "o3",
        workspace_id: "w1",
        deal_id: "d1",
        offer_number: "OFF-3",
        revision: 1,
        status: "draft",
        currency: "EUR",
        source: "test",
        captured_by: "human:test",
        created_at: "2026-07-01T00:00:00Z",
        updated_at: "2026-07-01T00:00:00Z",
        line_items: [],
        net_minor: 0,
        tax_minor: 0,
        gross_minor: 0,
      },
      error: undefined,
    });
    const { result } = renderHook(() => useCreateOffer("d1"), {
      wrapper: localWrapper,
    });
    await act(async () => {
      await result.current.mutateAsync({
        offer_number: "OFF-2",
        currency: "EUR",
        source: "test",
        captured_by: "human:test",
      });
    });
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/deals/{id}/offers",
      expect.objectContaining({
        params: { path: { id: "d1" } },
        headers: expect.objectContaining({
          "Idempotency-Key": "00000000-0000-0000-0000-000000000001",
        }),
      }),
    );
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: offersKeys.dealOffers("d1"),
    });
    await act(async () => {
      await result.current.mutateAsync({
        offer_number: "OFF-3",
        currency: "EUR",
        source: "test",
        captured_by: "human:test",
      });
    });
    expect(apiClient.POST).toHaveBeenNthCalledWith(
      2,
      "/deals/{id}/offers",
      expect.objectContaining({
        headers: expect.objectContaining({
          "Idempotency-Key": "00000000-0000-0000-0000-000000000002",
        }),
      }),
    );
    uuidSpy.mockRestore();
  });

  it("useCreateLineItem, useUpdateLineItem, and useDeleteLineItem invalidate both queries on settle", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "li3",
        workspace_id: "w1",
        offer_id: "o1",
        position: 3,
        description: "Third",
        unit: "unit",
        quantity: 1,
        unit_price_minor: 300,
        discount_pct: 0,
        tax_rate: 0,
        source: "test",
        captured_by: "human:test",
        created_at: "2026-07-01T00:00:00Z",
        updated_at: "2026-07-01T00:00:00Z",
        evidence: null,
        price_grounded: true,
      },
      error: undefined,
    });
    (apiClient.PATCH as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "li3",
        workspace_id: "w1",
        offer_id: "o1",
        position: 3,
        description: "Third edited",
        unit: "unit",
        quantity: 1,
        unit_price_minor: 350,
        discount_pct: 0,
        tax_rate: 0,
        source: "test",
        captured_by: "human:test",
        created_at: "2026-07-01T00:00:00Z",
        updated_at: "2026-07-01T00:00:00Z",
        evidence: null,
        price_grounded: true,
      },
      error: undefined,
    });
    (apiClient.DELETE as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: undefined,
      error: undefined,
    });

    const create = renderHook(() => useCreateLineItem("o1"), {
      wrapper: localWrapper,
    }).result;
    const update = renderHook(() => useUpdateLineItem("o1"), {
      wrapper: localWrapper,
    }).result;
    const remove = renderHook(() => useDeleteLineItem("o1"), {
      wrapper: localWrapper,
    }).result;

    await act(async () => {
      await create.current.mutateAsync({
        position: 3,
        description: "Third",
        unit: "unit",
        quantity: 1,
        unit_price_minor: 300,
        discount_pct: 0,
        tax_rate: 0,
        source: "test",
        captured_by: "human:test",
      });
      await update.current.mutateAsync({
        lineId: "li3",
        patch: { description: "Third edited", unit_price_minor: 350 },
      });
      await remove.current.mutateAsync({ lineId: "li3" });
    });

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: offersKeys.lineItems("o1"),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: offersKeys.detail("o1"),
    });
  });

  it("useRegenerateOffer returns the raw response and invalidates deal offers", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const response = {
      id: "o2",
      workspace_id: "w1",
      deal_id: "d1",
      offer_number: "OFF-1",
      revision: 2,
      status: "draft" as const,
      currency: "EUR",
      source: "agent:test",
      captured_by: "agent:test",
      created_at: "2026-07-01T00:00:00Z",
      updated_at: "2026-07-01T00:00:00Z",
      line_items: [],
      net_minor: 0,
      tax_minor: 0,
      gross_minor: 0,
      ai_generated: true,
      ai_disclosure: "AI disclosure",
      diff_from_previous: { added: [], removed: [], changed: [] },
    };
    vi.spyOn(globalThis.crypto, "randomUUID").mockReturnValue(
      "00000000-0000-0000-0000-0000000000aa",
    );
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: response,
      error: undefined,
    });
    const { result } = renderHook(() => useRegenerateOffer("d1"), {
      wrapper: localWrapper,
    });
    const resolved = await result.current.mutateAsync({ offerId: "o1" });
    expect(resolved).toEqual(response);
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/offers/{id}/regenerate",
      expect.objectContaining({
        params: { path: { id: "o1" } },
        headers: expect.objectContaining({
          "Idempotency-Key": "00000000-0000-0000-0000-0000000000aa",
        }),
      }),
    );
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: offersKeys.dealOffers("d1"),
    });
  });

  it("useRenderOffer and useSendOffer update the cached offer detail", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const setQueryDataSpy = vi.spyOn(qc, "setQueryData");
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "o1",
        workspace_id: "w1",
        deal_id: "d1",
        offer_number: "OFF-1",
        revision: 1,
        status: "draft",
        currency: "EUR",
        source: "test",
        captured_by: "human:test",
        created_at: "2026-07-01T00:00:00Z",
        updated_at: "2026-07-01T00:00:00Z",
        line_items: [],
        net_minor: 0,
        tax_minor: 0,
        gross_minor: 0,
        pdf_asset_ref: "asset-1",
      },
      error: undefined,
    });
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "o1",
        workspace_id: "w1",
        deal_id: "d1",
        offer_number: "OFF-1",
        revision: 1,
        status: "sent",
        currency: "EUR",
        source: "test",
        captured_by: "human:test",
        created_at: "2026-07-01T00:00:00Z",
        updated_at: "2026-07-01T00:00:00Z",
        line_items: [],
        net_minor: 0,
        tax_minor: 0,
        gross_minor: 0,
      },
      error: undefined,
    });
    const render = renderHook(() => useRenderOffer("o1"), {
      wrapper: localWrapper,
    }).result;
    const send = renderHook(() => useSendOffer("o1"), {
      wrapper: localWrapper,
    }).result;
    await act(async () => {
      await render.current.mutateAsync();
      await send.current.mutateAsync();
    });
    expect(setQueryDataSpy).toHaveBeenCalledWith(
      offersKeys.detail("o1"),
      expect.objectContaining({ pdf_asset_ref: "asset-1" }),
    );
    expect(setQueryDataSpy).toHaveBeenCalledWith(
      offersKeys.detail("o1"),
      expect.objectContaining({ status: "sent" }),
    );
  });

  it("never sends an X-Approval-Token header", async () => {
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: undefined,
      error: undefined,
    });
    const { result } = renderHook(() => useSendOffer("o1"), { wrapper });
    await act(async () => {
      try {
        await result.current.mutateAsync();
      } catch {
        // ignore
      }
    });
    expect(apiClient.POST).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        headers: expect.objectContaining({
          "X-Approval-Token": expect.anything(),
        }),
      }),
    );
  });
});
