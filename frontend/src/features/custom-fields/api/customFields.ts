import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type {
  CreateCustomFieldRequest,
  CustomField,
  CustomFieldListResponse,
} from "../../../lib/api-client/generated/index.js";
import type { ObjectKey } from "../lib/customFieldRules.js";

export const customFieldsKeys = {
  all: ["custom-fields"] as const,
  list: (object: ObjectKey) => ["custom-fields", "list", object] as const,
};

export function useCustomFields(object: ObjectKey) {
  return useQuery<CustomFieldListResponse>({
    queryKey: customFieldsKeys.list(object),
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/custom-fields", {
        params: { query: { object } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function useCreateCustomField() {
  const qc = useQueryClient();
  return useMutation<CustomField, unknown, CreateCustomFieldRequest>({
    mutationFn: async (body) => {
      const { data, error } = await apiClient.POST("/custom-fields", { body });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (field) => {
      qc.invalidateQueries({
        queryKey: customFieldsKeys.list(field.object),
      });
    },
  });
}

export function useRenameCustomField() {
  const qc = useQueryClient();
  return useMutation<CustomField, unknown, { id: string; label: string }>({
    mutationFn: async ({ id, label }) => {
      const { data, error } = await apiClient.PATCH("/custom-fields/{id}", {
        params: { path: { id } },
        body: { label },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (field) => {
      qc.invalidateQueries({
        queryKey: customFieldsKeys.list(field.object),
      });
    },
  });
}

export function useRetireCustomField() {
  const qc = useQueryClient();
  return useMutation<CustomField, unknown, string>({
    mutationFn: async (id) => {
      const { data, error } = await apiClient.POST(
        "/custom-fields/{id}/retire",
        {
          params: { path: { id } },
        },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (field) => {
      qc.invalidateQueries({
        queryKey: customFieldsKeys.list(field.object),
      });
    },
  });
}

export function useUpdateCustomFieldOptions() {
  const qc = useQueryClient();
  return useMutation<CustomField, unknown, { id: string; options: string[] }>({
    mutationFn: async ({ id, options }) => {
      const { data, error } = await apiClient.PATCH(
        "/custom-fields/{id}/options",
        {
          params: { path: { id } },
          body: { options },
        },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (field) => {
      qc.invalidateQueries({
        queryKey: customFieldsKeys.list(field.object),
      });
    },
  });
}

export type { CustomField };
