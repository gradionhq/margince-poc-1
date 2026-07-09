import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type {
  CreateOfferLineItemRequest,
  CreateOfferRequest,
  Offer,
  OfferLineItem,
  OfferListResponse,
  UpdateOfferLineItemRequest,
} from "../../../lib/api-client/generated/index.js";

export const offersKeys = {
  dealOffers: (dealId?: string) => ["offers", "deal", dealId] as const,
  detail: (offerId?: string) => ["offers", "detail", offerId] as const,
  lineItems: (offerId?: string) => ["offers", "lineItems", offerId] as const,
};

function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

export function useDealOffers(dealId: string | undefined) {
  return useQuery<OfferListResponse>({
    queryKey: offersKeys.dealOffers(dealId),
    enabled: !!dealId,
    queryFn: async () => {
      const { data, error } = await apiClient.GET(
        `/deals/${dealId}/offers` as "/deals/{id}/offers",
        { params: { path: { id: dealId as string } } },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function useOffer(offerId: string | undefined) {
  return useQuery<Offer>({
    queryKey: offersKeys.detail(offerId),
    enabled: !!offerId,
    queryFn: async () => {
      const { data, error } = await apiClient.GET(
        `/offers/${offerId}` as "/offers/{id}",
        { params: { path: { id: offerId as string } } },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function useOfferLineItems(offerId: string | undefined) {
  return useQuery<OfferLineItem[]>({
    queryKey: offersKeys.lineItems(offerId),
    enabled: !!offerId,
    queryFn: async () => {
      const { data, error } = await apiClient.GET(
        `/offers/${offerId}/line-items` as "/offers/{id}/line-items",
        { params: { path: { id: offerId as string } } },
      );
      if (error) throw error;
      return (data?.data ?? []).slice().sort((a, b) => a.position - b.position);
    },
  });
}

export function useCreateOffer(dealId: string | undefined) {
  const qc = useQueryClient();
  return useMutation<Offer, unknown, CreateOfferRequest>({
    mutationFn: async (body) => {
      const { data, error } = await apiClient.POST("/deals/{id}/offers", {
        params: { path: { id: dealId as string } },
        body,
        headers: { "Idempotency-Key": newIdempotencyKey() },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: offersKeys.dealOffers(dealId) });
    },
  });
}

function invalidateOfferReads(
  qc: ReturnType<typeof useQueryClient>,
  offerId: string,
) {
  qc.invalidateQueries({ queryKey: offersKeys.lineItems(offerId) });
  qc.invalidateQueries({ queryKey: offersKeys.detail(offerId) });
}

export function useCreateLineItem(offerId: string | undefined) {
  const qc = useQueryClient();
  return useMutation<OfferLineItem, unknown, CreateOfferLineItemRequest>({
    mutationFn: async (body) => {
      const { data, error } = await apiClient.POST("/offers/{id}/line-items", {
        params: { path: { id: offerId as string } },
        body,
        headers: { "Idempotency-Key": newIdempotencyKey() },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSettled: () => {
      invalidateOfferReads(qc, offerId as string);
    },
  });
}

export function useUpdateLineItem(offerId: string | undefined) {
  const qc = useQueryClient();
  return useMutation<
    OfferLineItem,
    unknown,
    { lineId: string; patch: UpdateOfferLineItemRequest }
  >({
    mutationFn: async ({ lineId, patch }) => {
      const { data, error } = await apiClient.PATCH(
        "/offers/{id}/line-items/{lineId}",
        {
          params: { path: { id: offerId as string, lineId } },
          body: patch,
          headers: { "Idempotency-Key": newIdempotencyKey() },
        },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSettled: () => {
      invalidateOfferReads(qc, offerId as string);
    },
  });
}

export function useDeleteLineItem(offerId: string | undefined) {
  const qc = useQueryClient();
  return useMutation<void, unknown, { lineId: string }>({
    mutationFn: async ({ lineId }) => {
      const { error } = await apiClient.DELETE(
        "/offers/{id}/line-items/{lineId}",
        {
          params: { path: { id: offerId as string, lineId } },
          headers: { "Idempotency-Key": newIdempotencyKey() },
        },
      );
      if (error) throw error;
    },
    onSettled: () => {
      invalidateOfferReads(qc, offerId as string);
    },
  });
}

export function useRegenerateOffer(dealId: string | undefined) {
  const qc = useQueryClient();
  return useMutation<Offer, unknown, { offerId: string }>({
    mutationFn: async ({ offerId }) => {
      const { data, error } = await apiClient.POST("/offers/{id}/regenerate", {
        params: { path: { id: offerId } },
        headers: { "Idempotency-Key": newIdempotencyKey() },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (_data) => {
      qc.invalidateQueries({ queryKey: offersKeys.dealOffers(dealId) });
    },
  });
}

export function useRenderOffer(offerId: string | undefined) {
  const qc = useQueryClient();
  return useMutation<Offer, unknown, void>({
    mutationFn: async () => {
      const { data, error } = await apiClient.POST("/offers/{id}/render", {
        params: { path: { id: offerId as string } },
        headers: { "Idempotency-Key": newIdempotencyKey() },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (data) => {
      qc.setQueryData(offersKeys.detail(offerId), data);
    },
  });
}

export function useSendOffer(offerId: string | undefined) {
  const qc = useQueryClient();
  return useMutation<Offer, unknown, void>({
    mutationFn: async () => {
      const { data, error } = await apiClient.POST("/offers/{id}/send", {
        params: { path: { id: offerId as string } },
        headers: { "Idempotency-Key": newIdempotencyKey() },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (data) => {
      qc.setQueryData(offersKeys.detail(offerId), data);
    },
  });
}
