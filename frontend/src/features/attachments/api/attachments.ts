import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type { components } from "../../../lib/api-client/generated/index.js";
import { dealsKeys } from "../../deals/api/deals.js";

type Attachment = components["schemas"]["Attachment"];
type AttachmentExtraction = components["schemas"]["AttachmentExtraction"];
type CreateAttachmentRequest = components["schemas"]["CreateAttachmentRequest"];
type AcceptExtractionRequest = components["schemas"]["AcceptExtractionRequest"];
type AttachmentExtractionAcceptResponse =
  components["schemas"]["AttachmentExtractionAcceptResponse"];
type RequestAccessResponse = components["schemas"]["RequestAccessResponse"];

export type CreateAttachmentInput = {
  request: CreateAttachmentRequest;
  file: File;
};

export const attachmentsKeys = {
  all: ["attachments"] as const,
  list: (entityType?: string, entityId?: string) =>
    ["attachments", "list", entityType, entityId] as const,
  detail: (id?: string) => ["attachments", "detail", id] as const,
  extraction: (id?: string) => ["attachments", "extraction", id] as const,
};

async function putAttachmentBytes(url: string, file: File) {
  const response = await fetch(url, {
    method: "PUT",
    body: file,
    headers: file.type ? { "Content-Type": file.type } : undefined,
  });
  if (!response.ok) {
    throw new Error("failed to upload attachment bytes");
  }
}

export function useAttachments(filter: {
  entityType: string;
  entityId: string;
}) {
  return useQuery<Attachment[]>({
    queryKey: attachmentsKeys.list(filter.entityType, filter.entityId),
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/attachments", {
        params: {
          query: {
            entity_type: filter.entityType as
              | "person"
              | "organization"
              | "deal"
              | "lead"
              | "activity",
            entity_id: filter.entityId,
          },
        },
      });
      if (error) throw error;
      return data?.data ?? [];
    },
    refetchInterval: (query) => {
      const attachments = query.state.data ?? [];
      return attachments.some(
        (attachment) => attachment.scan_status === "scanning",
      )
        ? 3000
        : false;
    },
    refetchIntervalInBackground: true,
  });
}

export function useAttachment(id: string | undefined) {
  return useQuery<Attachment>({
    queryKey: attachmentsKeys.detail(id),
    enabled: !!id,
    queryFn: async () => {
      const { data, error } = await apiClient.GET(
        `/attachments/${id}` as "/attachments/{id}",
        {
          params: { path: { id: id as string } },
        },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function useCreateAttachment() {
  const queryClient = useQueryClient();
  return useMutation<Attachment, unknown, CreateAttachmentInput>({
    mutationFn: async ({ request, file }) => {
      const { data, error } = await apiClient.POST("/attachments", {
        body: request,
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      if (data.upload_url) {
        await putAttachmentBytes(data.upload_url, file);
      }
      return data;
    },
    onSuccess: (attachment) => {
      queryClient.invalidateQueries({
        queryKey: attachmentsKeys.list(
          attachment.entity_type,
          attachment.entity_id,
        ),
      });
      queryClient.invalidateQueries({
        queryKey: attachmentsKeys.detail(attachment.id),
      });
    },
  });
}

export function useArchiveAttachment(id: string) {
  const queryClient = useQueryClient();
  return useMutation<Attachment, unknown, void>({
    mutationFn: async () => {
      const { data, error } = await apiClient.DELETE(
        `/attachments/${id}` as "/attachments/{id}",
        {
          params: { path: { id } },
        },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (attachment) => {
      queryClient.invalidateQueries({
        queryKey: attachmentsKeys.list(
          attachment.entity_type,
          attachment.entity_id,
        ),
      });
      queryClient.invalidateQueries({
        queryKey: attachmentsKeys.detail(attachment.id),
      });
    },
  });
}

export function useAttachmentExtraction(id: string | undefined) {
  return useQuery<AttachmentExtraction>({
    queryKey: attachmentsKeys.extraction(id),
    enabled: !!id,
    queryFn: async () => {
      const { data, error } = await apiClient.GET(
        `/attachments/${id}/extraction` as "/attachments/{id}/extraction",
        {
          params: { path: { id: id as string } },
        },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function useAcceptExtraction(id: string) {
  const queryClient = useQueryClient();
  return useMutation<
    AttachmentExtractionAcceptResponse,
    unknown,
    AcceptExtractionRequest
  >({
    mutationFn: async (body) => {
      const { data, error } = await apiClient.POST(
        `/attachments/${id}/extraction:accept` as "/attachments/{id}/extraction:accept",
        {
          params: { path: { id } },
          body,
        },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({
        queryKey: attachmentsKeys.detail(id),
      });
      queryClient.invalidateQueries({
        queryKey: attachmentsKeys.extraction(id),
      });
      queryClient.invalidateQueries({
        queryKey: dealsKeys.detail(result.deal_id),
      });
    },
  });
}

export function useRequestAccess(id: string) {
  return useMutation<RequestAccessResponse, unknown, void>({
    mutationFn: async () => {
      const { data, error } = await apiClient.POST(
        `/attachments/${id}/request-access` as "/attachments/{id}/request-access",
        {
          params: { path: { id } },
        },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}
