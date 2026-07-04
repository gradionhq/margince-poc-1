import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type {
  Person,
  PersonListResponse,
} from "../../../lib/api-client/generated/index.js";

export function usePeople(opts?: { sort?: string; q?: string }) {
  return useQuery<PersonListResponse>({
    queryKey: ["people", opts?.sort, opts?.q],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/people", {
        params: { query: { sort: opts?.sort, q: opts?.q } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function usePerson(id: string | undefined) {
  return useQuery<Person>({
    queryKey: ["people", "detail", id],
    enabled: !!id,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/people/{id}", {
        params: { path: { id: id as string } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}
